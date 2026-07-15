package agent

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadFile(name string) (*Config, error) {
	bytes, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	cfg, err := load(os.ExpandEnv(string(bytes)))
	if err != nil {
		fmt.Printf("parsing yaml file err:%v", err)
		return nil, err
	}
	ls := SetLogRegs(cfg.LogStrategies)
	cfg.LogStrategies = ls
	return cfg, nil
}

func load(in string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(in), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
