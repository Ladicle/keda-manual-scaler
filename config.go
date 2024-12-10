package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	GrpcPort int `yaml:"grpcPort"`
	HttpPort int `yaml:"httpPort"`
}

func parseConfig(path string) (*config, error) {
	var config config
	b, err := os.ReadFile(path)
	if err != nil {
		return &config, fmt.Errorf("config: read: %s", err)
	}

	if err = yaml.Unmarshal(b, &config); err != nil {
		return &config, fmt.Errorf("config: unmarshal: %s", err)
	}
	return &config, nil
}
