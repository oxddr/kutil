package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/oxddr/kutil/kup/preset"
)

var (
	debug    = flag.Bool("debug", false, "debug mode")
	build    = flag.Bool("build", false, "Whether to include build command")
	size     = flag.Int("size", 3, "Size of the cluster")
	project  = flag.String("project", "k8s-scale-testing", "Name of the GCP project")
	provider = flag.String("provider", "gce", "Name of the provider [supported: gce, gke, kubemark]")
	name     = flag.String("name", "", "Name of the cluster")
	output   = flag.String("output", "$HOME/debug", "Parent directory for output")
	timeout  = flag.String("timeout", "", "Test timeout")
	up       = flag.Bool("up", true, "Whether to create a cluster")
	density  = flag.Bool("density", false, "Whether to run density tests")
	load     = flag.Bool("load", false, "Whether to run load tests")
	zone     = flag.String("zone", "us-east1-b", "Which GCP zone to run")
	userEnv  = flag.String("env", "", "Additional environmental variables to pass")
)

type env struct {
	name  string
	value string
}

type kup struct {
	cmds      []string
	extraArgs []string

	extraEnv []env

	prepared bool

	debug    bool
	build    bool
	size     int
	project  string
	provider string
	zone     string
	name     string
	output   string
	timeout  string
	up       bool
	density  bool
	load     bool
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

func (k *kup) NodesForKubemark() int {
	// TODO(oxddr): implement sizing logic in here
	return 83
}

func (k *kup) RealProvider() string {
	if k.provider == "kubemark" {
		return "gce"
	}
	return k.provider

}

func (k *kup) Timeout() string {
	if k.timeout != "" {
		return k.timeout
	}
	return "1290m"
}

func (k *kup) addArg(name, value string) {
	k.extraArgs = append(k.extraArgs, formatArg(name, value))
}

func (k *kup) addIntArg(name string, value int) {
	k.extraArgs = append(k.extraArgs, formatIntArg(name, value))
}

func (k *kup) addSwitch(name string) {
	k.extraArgs = append(k.extraArgs, fmt.Sprintf("--%s", name))
}

func (k *kup) exportEnv(name, value string) {
	k.addCmd(fmt.Sprintf("export %s=\"%s\"", name, value))
}

func (k *kup) applyPreset(presetName string) error {
	k.addCmd(fmt.Sprintf("# preset: %s", presetName))
	presetEnv, err := preset.ReadPresetEnv(presetName)
	if err != nil {
		return err
	}
	for _, env := range presetEnv {
		k.exportEnv(env.Name, env.Value)
	}
	k.addEmptyLine()
	return nil
}

func (k *kup) addEmptyLine() {
	k.addCmd("")
}

func (k *kup) addCmd(cmd string) {
	if *debug {
		cmd = fmt.Sprintf("echo '%s'", cmd)
	}
	k.cmds = append(k.cmds, cmd)
}

func (k *kup) maybeCreateOutput() {
	if k.up || k.load || k.density {
		k.addCmd(fmt.Sprintf("export OUTPUT=\"%s/run-$(date +%%m%%d-%%H%%M%%S)\"", k.output))
		k.addCmd("mkdir -p \"$OUTPUT\"")
		k.addEmptyLine()
	}
}

func (k *kup) maybeBuild() {
	if k.build {
		k.addCmd("make quick-release")
		k.addEmptyLine()
	}
}

func (k *kup) addExtraEnv() {
	if len(k.extraEnv) == 0 {
		return
	}

	k.addCmd("# user-specified variables")
	for _, env := range k.extraEnv {
		k.exportEnv(env.name, env.value)
	}
	k.addEmptyLine()
}

func (k *kup) processExtraArgs() {
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
			k.addArg("gcp-node", "n1-standard-1")
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
	}

	k.extraArgs = append(k.extraArgs, flag.Args()...)

	if k.up || k.load || k.density {
		k.extraArgs = append(k.extraArgs, "2>&1 | tee \"$OUTPUT/build-log.txt\"")
	}
}

func (k *kup) prepareCommands() error {
	k.maybeCreateOutput()
	k.maybeBuild()

	if k.provider == "gce" && k.size == 5000 {
		k.exportEnv("HEAPSTER_MACHINE_TYPE", "n1-standard-32")
		k.addEmptyLine()
	}

	if err := k.applyPreset("preset-e2e-scalability-common"); err != nil {
		return err
	}

	if k.provider == "kubemark" {
		if err := k.applyPreset("preset-e2e-kubemark-common"); err != nil {
			return err
		}
		if err := k.applyPreset("preset-e2e-kubemark-gce-scale"); err != nil {
			return err
		}

		// KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by
		// the kubemark creation script. This is a temporary workaround.
		k.addCmd("# KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by")
		k.addCmd("# the kubemark creation script. This is a temporary workaround.")
		k.exportEnv("KUBE_GCE_NETWORK", "e2e-${USER}")
		k.exportEnv("INSTANCE_PREFIX", "e2e-${USER}")
		k.exportEnv("KUBE_GCE_INSTANCE_PREFIX", "e2e-${USER}")
		k.addEmptyLine()
	}

	k.addExtraEnv()
	k.processExtraArgs()
	// TODO(oxddr): add an option to use bare kubetest
	k.addCmd(fmt.Sprintf("go run hack/e2e.go -v -- \\\n  %s", strings.Join(k.extraArgs, " \\\n  ")))
	return nil
}

func (k *kup) GetCommands() (string, error) {
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
	flag.Parse()

	extraEnv, err := parseUserEnv(*userEnv)
	if err != nil {
		handleErr(err)
	}

	kup := &kup{
		build:    *build,
		size:     *size,
		project:  *project,
		provider: *provider,
		name:     *name,
		output:   *output,
		timeout:  *timeout,
		up:       *up,
		density:  *density,
		load:     *load,
		zone:     *zone,
		extraEnv: extraEnv,
	}
	cmds, err := kup.GetCommands()

	if err != nil {
		handleErr(err)
	}

	fmt.Println(cmds)
}
