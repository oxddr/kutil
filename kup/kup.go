package main

import (
	"flag"
	"fmt"
	"strings"
)

const (
	setCommonEnv = "$HOME/src/github.com/mm4tt/k8s-util/set-common-envs/set-common-envs.sh"
)

var kubeUp KubeUp

func kubeUpFlags() {
	flag.BoolVar(&kubeUp.density, "density", false, "Whether to run density tests")
	flag.BoolVar(&kubeUp.load, "load", false, "Whether to run load tests")
	flag.BoolVar(&kubeUp.up, "up", true, "Whether to create a cluster")
	flag.BoolVar(&kubeUp.build, "build", true, "Whether to include build command")
	flag.BoolVar(&kubeUp.prometheus, "prometheus", true, "Whether to enable Prometheus and keep it running once tests are finished")
	flag.IntVar(&kubeUp.size, "size", 3, "Size of the cluster")
	flag.StringVar(&kubeUp.timeout, "timeout", "", "Test timeout")
	flag.StringVar(&kubeUp.zone, "zone", "europe-north1-a", "Which GCP zone to run")
	flag.StringVar(&kubeUp.name, "name", "", "Name of the cluster")
	flag.StringVar(&kubeUp.output, "output", "$HOME/debug", "Parent directory for output")
	flag.StringVar(&kubeUp.project, "project", "", "Name of the GCP project")
	flag.StringVar(&kubeUp.provider, "provider", "gce", "Name of the provider [supported: gce, gke, kubemark]")
	flag.StringVar(&kubeUp.testInfraCommit, "test-infra-commit", "", "Commit to be used to load presets")
	flag.BoolVar(&kubeUp.debug, "debug", false, "debug mode")
	flag.BoolVar(&kubeUp.private, "private", true, "use private cluster")
	flag.Var(&kubeUp.extraEnv, "env", "Additional environmental variables to pass")
}

type env struct {
	name  string
	value string
}

type envSlice []env

func (s *envSlice) String() string {
	return fmt.Sprintf("%s", *s)
}

func (s *envSlice) Set(value string) error {
	tokens := strings.Split(value, "=")
	if len(tokens) != 2 {
		return fmt.Errorf("malformed env: %s", value)
	}
	*s = append(*s, env{name: tokens[0], value: tokens[1]})
	return nil
}

func formatArg(name, value string) string {
	return fmt.Sprintf("--%s=%s", name, value)
}

func formatIntArg(name string, value int) string {
	return fmt.Sprintf("--%s=%d", name, value)
}

type command struct {
	base string
	args []string
}

func (c *command) String() string {
	return fmt.Sprintf("%s \\\n  %s", c.base, strings.Join(c.args, " \\\n  "))
}

func newCommand(cmd string) command {
	return command{base: cmd, args: make([]string, 0)}
}

func (c *command) AddArg(name, value string) {
	c.args = append(c.args, formatArg(name, value))
}

func (c *command) AddIntArg(name string, value int) {
	c.args = append(c.args, formatIntArg(name, value))
}

func (c *command) AddSwitch(name string) {
	c.args = append(c.args, fmt.Sprintf("--%s", name))
}

func (c *command) AddRaw(str string) {
	c.args = append(c.args, str)
}

type scriptBuilder struct {
	cmds  []string
	debug bool
}

func newBuilder(debug bool) *scriptBuilder {
	return &scriptBuilder{debug: debug, cmds: make([]string, 0)}
}

func (b *scriptBuilder) AddCmd(cmd string) {
	if b.debug {
		cmd = fmt.Sprintf("echo '%s'", cmd)
	}
	if b.cmds == nil {
		b.cmds = make([]string, 10)
	}
	b.cmds = append(b.cmds, cmd)
}

func (b *scriptBuilder) AddEmptyLine() {
	b.AddCmd("")
}

func (b *scriptBuilder) ExportEnv(name, value string) {
	b.AddCmd(fmt.Sprintf("export %s=\"%s\"", name, value))
}

func (b *scriptBuilder) String() string {
	var str strings.Builder
	for _, c := range b.cmds {
		str.WriteString(fmt.Sprintf("%s\n", c))
	}
	return str.String()
}

type KubeUp struct {
	builder *scriptBuilder

	extraEnv        envSlice
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

func (k *KubeUp) applyPreset(presetName string) {
	k.builder.AddCmd(fmt.Sprintf("# preset: %s", presetName))
	k.builder.AddCmd(fmt.Sprintf("source %s %s %s", setCommonEnv, presetName, k.testInfraCommit))
	k.builder.AddEmptyLine()
}

func (k *KubeUp) maybeCreateOutput() {
	if k.up || k.load || k.density {
		k.builder.AddCmd(fmt.Sprintf("export OUTPUT=\"%s/run-$(date +%%m%%d-%%H%%M%%S)\"", k.output))
		k.builder.AddCmd("mkdir -p \"$OUTPUT\"")
		k.builder.AddEmptyLine()
	}
}

func (k *KubeUp) maybeBuild() {
	if k.build {
		k.builder.AddCmd("make quick-release")
		k.builder.AddEmptyLine()
	}
}

func (k *KubeUp) addExtraEnv() {
	if len(k.extraEnv) == 0 {
		return
	}

	k.builder.AddCmd("# user-specified variables")
	for _, env := range k.extraEnv {
		k.builder.ExportEnv(env.name, env.value)
	}
	k.builder.AddEmptyLine()
}

func (k *KubeUp) buildRunCommand() {
	cmd := newCommand("go run hack/e2e.go -v --")

	cmd.AddArg("provider", k.RealProvider())
	cmd.AddArg("gcp-project", k.project)
	cmd.AddArg("gcp-zone", k.zone)

	if k.up {
		cmd.AddSwitch("up")
		switch k.provider {
		case "gke":
			cmd.AddArg("deployment", "gke")
			cmd.AddArg("gcp-network", k.name)
			cmd.AddArg("gcp-node-image", "gci")
			cmd.AddArg("gke-environment", "prod")
			cmd.AddArg("gke-shape", "{\"default\":{\"Nodes\":$size,\"MachineType\":\"n1-standard-1\"}}")
		case "gce":
			cmd.AddArg("gcp-node-size", "n1-standard-1")
			cmd.AddIntArg("gcp-nodes", k.size)
		case "kubemark":
			cmd.AddArg("gcp-node-image", "gci")
			cmd.AddArg("gcp-node-size", "n1-standard-8")
			cmd.AddIntArg("gcp-nodes", k.NodesForKubemark())
			cmd.AddIntArg("kubemark-nodes", k.size)
			cmd.AddSwitch("kubemark")
		}
	}

	if k.load || k.density {
		cmd.AddArg("test", "false")
		cmd.AddArg("test-cmd-name", "ClusterLoaderV2")
		cmd.AddArg("test-cmd", "$GOPATH/src/k8s.io/perf-tests/run-e2e.sh")
		cmd.AddArg("test-cmd-args", "cluster-loader2")
		cmd.AddArg("test-cmd-args", formatIntArg("nodes", k.size))
		cmd.AddArg("test-cmd-args", formatArg("provider", k.provider))
		cmd.AddArg("test-cmd-args", formatArg("report-dir", "\"$OUTPUT/_artifacts\""))

		if k.load {
			cmd.AddArg("test-cmd-args", formatArg("testconfig", "testing/load/config.yaml"))
		}
		if k.density {
			cmd.AddArg("test-cmd-args", formatArg("testconfig", "testing/density/config.yaml"))
		}
		if k.density && k.size == 5000 {
			cmd.AddArg("test-cmd-args", formatArg("testoverrides", "testing/density/5000_nodes/override.yaml"))
		}
		if k.Timeout() != "" {
			cmd.AddArg("timeout", k.Timeout())
		}
		if k.prometheus {
			cmd.AddArg("test-cmd-args", formatArg("enable-prometheus-server", "true"))
			cmd.AddArg("test-cmd-args", formatArg("tear-down-prometheus-server", "false"))
			cmd.AddArg("test-cmd-args", formatArg("experimental-gcp-snapshot-prometheus-disk", "true"))
		}
	}

	for _, a := range flag.Args() {
		cmd.AddRaw(a)
	}

	if k.up || k.load || k.density {
		cmd.AddRaw("2>&1 | tee \"$OUTPUT/build-log.txt\"")
	}
	k.builder.AddCmd(cmd.String())

}

func (k *KubeUp) String() string {
	k.builder = newBuilder(k.debug)

	k.builder.AddCmd("set -e")
	k.maybeCreateOutput()
	k.maybeBuild()

	// temp dev variables
	k.builder.ExportEnv("KUBEPROXY_TEST_LOG_LEVEL", "--v=4")
	k.builder.ExportEnv("HEAPSTER_MACHINE_TYPE", "n1-standard-16")
	k.builder.ExportEnv("PROMETHEUS_SCRAPE_KUBELETS", "true")

	if k.up {
		if k.provider == "gce" && k.size >= 2000 {
			k.builder.ExportEnv("HEAPSTER_MACHINE_TYPE", "n1-standard-32")
			k.builder.AddEmptyLine()
		}

		k.applyPreset("preset-e2e-scalability-common")

		if k.provider == "kubemark" {
			k.applyPreset("preset-e2e-kubemark-common")
			k.applyPreset("preset-e2e-kubemark-gce-scale")

			// KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by
			// the kubemark creation script. This is a temporary workaround.
			k.builder.AddCmd("# KUBE_GCE_NETWORK and INSTANCE_PREFIX are required by")
			k.builder.AddCmd("# the kubemark creation script. This is a temporary workaround.")
			k.builder.ExportEnv("KUBE_GCE_NETWORK", "e2e-${USER}")
			k.builder.ExportEnv("INSTANCE_PREFIX", "e2e-${USER}")
			k.builder.ExportEnv("KUBE_GCE_INSTANCE_PREFIX", "e2e-${USER}")
			k.builder.AddEmptyLine()
		}
	}

	k.addExtraEnv()

	if k.private {
		k.builder.ExportEnv("KUBE_GCE_PRIVATE_CLUSTER", "true")
	}

	k.buildRunCommand()
	k.builder.AddCmd("echo Results in $OUTPUT")
	return k.builder.String()
}

func main() {
	kubeUpFlags()
	flag.Parse()
	fmt.Println(kubeUp.String())
}
