package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	setCommonEnv = "$HOME/src/github.com/mm4tt/k8s-util/set-common-envs/set-common-envs.sh"
)

var (
	kubeUp  KubeUp
	userEnv string
)

func initFlags() {
	flag.BoolVar(&kubeUp.density, "density", false, "Whether to run density tests")
	flag.BoolVar(&kubeUp.load, "load", false, "Whether to run load tests")
	flag.BoolVar(&kubeUp.up, "up", true, "Whether to create a cluster")
	flag.BoolVar(&kubeUp.build, "build", true, "Whether to include build command")
	flag.BoolVar(&kubeUp.stable, "stable", false, "Whether to use perf-tests or perf-tests-dev")
	flag.BoolVar(&kubeUp.prometheus, "prometheus", true, "Whether to enable Prometheus and keep it running once tests are finished")
	flag.IntVar(&kubeUp.size, "size", 3, "Size of the cluster")
	flag.StringVar(&kubeUp.timeout, "timeout", "", "Test timeout")
	flag.StringVar(&kubeUp.zone, "zone", "europe-north1-a", "Which GCP zone to run")
	flag.StringVar(&kubeUp.name, "name", "", "Name of the cluster")
	flag.StringVar(&kubeUp.output, "output", "$HOME/debug", "Parent directory for output")
	flag.StringVar(&kubeUp.project, "project", "k8s-scale-testing", "Name of the GCP project")
	flag.StringVar(&kubeUp.provider, "provider", "gce", "Name of the provider [supported: gce, gke, kubemark]")
	flag.StringVar(&kubeUp.testInfraCommit, "test-infra-commit", "", "Commit to be used to load presets")
	flag.BoolVar(&kubeUp.debug, "debug", false, "debug mode")
	flag.BoolVar(&kubeUp.private, "private", true, "use private cluster")

	flag.StringVar(&userEnv, "env", "", "Additional environmental variables to pass")

	flag.Parse()
}

type env struct {
	name  string
	value string
}

type KubeUp struct {
	cmds      []string
	extraArgs []string

	extraEnv []env

	prepared bool

	debug           bool
	build           bool
	size            int
	project         string
	provider        string
	zone            string
	name            string
	output          string
	timeout         string
	testInfraCommit string
	private         bool
	up              bool
	density         bool
	load            bool
	prometheus      bool
}

func formatArg(name, value string) string {
	return fmt.Sprintf("--%s=%s", name, value)
}

func formatIntArg(name string, value int) string {
	return fmt.Sprintf("--%s=%d", name, value)
}

func parseUserEnv(envString string) ([]env, error) {
	if envString == "" {
		return make([]env, 0), nil
	}

	envList := strings.Split(envString, ",")
	var result []env
	for _, e := range envList {
		pair := strings.Split(e, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("malformed env passed: %s", envString)
		}
		result = append(result, env{name: pair[0], value: pair[1]})

	}
	return result, nil
}

func (k *KubeUp) NodesForKubemark() int {
	// TODO(oxddr): implement sizing logic in here
	return 83
}

func (k *KubeUp) RealProvider() string {
	if k.provider == "kubemark" {
		return "gce"
	}
	return k.provider

}

func (k *KubeUp) Timeout() string {
	if k.timeout != "" {
		return k.timeout
	}
	return "1290m"
}

func (k *KubeUp) addArg(name, value string) {
	k.extraArgs = append(k.extraArgs, formatArg(name, value))
}

func (k *KubeUp) addIntArg(name string, value int) {
	k.extraArgs = append(k.extraArgs, formatIntArg(name, value))
}

func (k *KubeUp) addSwitch(name string) {
	k.extraArgs = append(k.extraArgs, fmt.Sprintf("--%s", name))
}

func (k *KubeUp) exportEnv(name, value string) {
	k.addCmd(fmt.Sprintf("export %s=\"%s\"", name, value))
}

func (k *KubeUp) applyPreset(presetName string) {
	k.addCmd(fmt.Sprintf("# preset: %s", presetName))
	k.addCmd(fmt.Sprintf("source %s %s %s", setCommonEnv, presetName, k.testInfraCommit))
	k.addEmptyLine()
}

func (k *KubeUp) addEmptyLine() {
	k.addCmd("")
}

func (k *KubeUp) addCmd(cmd string) {
	if k.debug {
		cmd = fmt.Sprintf("echo '%s'", cmd)
	}
	k.cmds = append(k.cmds, cmd)
}

func (k *KubeUp) maybeCreateOutput() {
	if k.up || k.load || k.density {
		k.addCmd(fmt.Sprintf("export OUTPUT=\"%s/run-$(date +%%m%%d-%%H%%M%%S)\"", k.output))
		k.addCmd("mkdir -p \"$OUTPUT\"")
		k.addEmptyLine()
	}
}

func (k *KubeUp) maybeBuild() {
	if k.build {
		k.addCmd("make quick-release")
		k.addEmptyLine()
	}
}

func (k *KubeUp) addExtraEnv() {
	if len(k.extraEnv) == 0 {
		return
	}

	k.addCmd("# user-specified variables")
	for _, env := range k.extraEnv {
		k.exportEnv(env.name, env.value)
	}
	k.addEmptyLine()
}

func (k *KubeUp) processExtraArgs() {
	k.addArg("provider", k.RealProvider())
	k.addArg("gcp-project", k.project)
	k.addArg("gcp-zone", k.zone)

	if k.up {
		k.addSwitch("up")
		switch k.provider {
		case "gke":
			k.addArg("deployment", "gke")
			k.addArg("gcp-network", k.name)
			k.addArg("gcp-node-image", "gci")
			k.addArg("gke-environment", "prod")
			k.addArg("gke-shape", "{\"default\":{\"Nodes\":$size,\"MachineType\":\"n1-standard-1\"}}")
		case "gce":
			k.addArg("gcp-node-size", "n1-standard-1")
			k.addIntArg("gcp-nodes", k.size)
		case "kubemark":
			k.addArg("gcp-node-image", "gci")
			k.addArg("gcp-node-size", "n1-standard-8")
			k.addIntArg("gcp-nodes", k.NodesForKubemark())
			k.addIntArg("kubemark-nodes", k.size)
			k.addSwitch("kubemark")
		}
	}

	if k.load || k.density {
		k.addArg("test", "false")
		k.addArg("test-cmd-name", "ClusterLoaderV2")
		k.addArg("test-cmd", "$GOPATH/src/k8s.io/perf-tests/run-e2e.sh")
		k.addArg("test-cmd-args", "cluster-loader2")
		k.addArg("test-cmd-args", formatIntArg("nodes", k.size))
		k.addArg("test-cmd-args", formatArg("provider", k.provider))
		k.addArg("test-cmd-args", formatArg("report-dir", "\"$OUTPUT/_artifacts\""))

		if k.load {
			k.addArg("test-cmd-args", formatArg("testconfig", "testing/load/config.yaml"))
		}
		if k.density {
			k.addArg("test-cmd-args", formatArg("testconfig", "testing/density/config.yaml"))
		}
		if k.density && k.size == 5000 {
			k.addArg("test-cmd-args", formatArg("testoverrides", "testing/density/5000_nodes/override.yaml"))
		}
		if k.Timeout() != "" {
			k.addArg("timeout", k.Timeout())
		}
		if k.prometheus {
			k.addArg("test-cmd-args", formatArg("enable-prometheus-server", "true"))
			k.addArg("test-cmd-args", formatArg("tear-down-prometheus-server", "false"))
			k.addArg("test-cmd-args", formatArg("experimental-gcp-snapshot-prometheus-disk", "true"))
		}
	}

	k.extraArgs = append(k.extraArgs, flag.Args()...)

	if k.up || k.load || k.density {
		k.extraArgs = append(k.extraArgs, "2>&1 | tee \"$OUTPUT/build-log.txt\"")
	}
}

func (k *KubeUp) prepareCommands() error {
	k.addCmd("set -e")
	k.maybeCreateOutput()
	k.maybeBuild()

	if k.up {
		if k.provider == "gce" && k.size >= 2000 {
			k.exportEnv("HEAPSTER_MACHINE_TYPE", "n1-standard-32")
			k.addEmptyLine()
		}

		k.exportEnv("KUBE_GCE_WINDOWS_NODES", "false")

		k.applyPreset("preset-e2e-scalability-common")

		if k.provider == "kubemark" {
			k.applyPreset("preset-e2e-kubemark-common")
			k.applyPreset("preset-e2e-kubemark-gce-scale")

			// KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by
			// the kubemark creation script. This is a temporary workaround.
			k.addCmd("# KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by")
			k.addCmd("# the kubemark creation script. This is a temporary workaround.")
			k.exportEnv("KUBE_GCE_NETWORK", "e2e-${USER}")
			k.exportEnv("INSTANCE_PREFIX", "e2e-${USER}")
			k.exportEnv("KUBE_GCE_INSTANCE_PREFIX", "e2e-${USER}")
			k.addEmptyLine()
		}
	}

	if k.private {
		k.exportEnv("KUBE_GCE_PRIVATE_CLUSTER", "true")
	}

	k.addExtraEnv()
	k.processExtraArgs()
	// TODO(oxddr): add an option to use bare kubetest
	k.addCmd(fmt.Sprintf("go run hack/e2e.go -v -- \\\n  %s", strings.Join(k.extraArgs, " \\\n  ")))
	return nil
}

func (k *KubeUp) GetCommands() (string, error) {
	if !k.prepared {
		if err := k.prepareCommands(); err != nil {
			return "", err
		}
	}
	return strings.Join(k.cmds, "\n"), nil
}

func handleErr(err error) {
	fmt.Println(err)
	os.Exit(1)

}

func main() {
	initFlags()

	var err error
	kubeUp.extraEnv, err = parseUserEnv(userEnv)
	if err != nil {
		handleErr(err)
	}

	cmds, err := kubeUp.GetCommands()
	if err != nil {
		handleErr(err)
	}

	fmt.Println(cmds)
}
