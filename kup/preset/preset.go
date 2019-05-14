package preset

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
)

var fileUrl = flag.String("file-url", "https://raw.githubusercontent.com/kubernetes/test-infra/master/config/jobs/kubernetes/sig-scalability/sig-scalability-presets.yaml", "URL of the yaml file with presets to read")

type Env struct {
	Name  string `yaml:name`
	Value string `yaml:value`
}

type conf struct {
	Presets []preset `yaml:presets`
}

type preset struct {
	Labels map[string]string `yaml:labels`
	Env    []Env             `yaml:env`
}

func (p preset) getName() string {
	for name, _ := range p.Labels {
		return name
	}
	panic("Empty Labels!")
}

func ReadPresetEnv(presetName string) ([]Env, error) {
	resp, err := http.Get(*fileUrl)
	if err != nil {
		return nil, err
	}
	yamlFile, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	config := new(conf)
	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		return nil, err
	}

	for _, preset := range config.Presets {
		name := preset.getName()

		if name == presetName {
			return preset.Env, nil
		}
	}

	return nil, fmt.Errorf("preset %s not found", presetName)
}
