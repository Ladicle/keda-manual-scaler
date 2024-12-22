package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	GrpcScalerPort int `yaml:"grpcScalerPort"`
	HttpAPIPort    int `yaml:"httpAPIPort"`

	DefaultConfig metricConfig     `yaml:"default"`
	ObjectMetrics map[string]int64 `yaml:"metrics"`
}

type metricConfig struct {
	metricName  string `yaml:"metricName"`
	active      bool   `yaml:"active"`
	targetSize  int64  `yaml:"targetSize"`
	metricValue int64  `yaml:"metricValue"`
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
