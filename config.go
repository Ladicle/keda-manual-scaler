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
	MetricName  string `yaml:"metricName"`
	Active      bool   `yaml:"active"`
	TargetSize  int64  `yaml:"targetSize"`
	MetricValue int64  `yaml:"metricValue"`
}

// defaultConfig returns the in-binary default configuration so the scaler can
// run without providing an external config file.
func defaultConfig() *config {
	return &config{
		GrpcScalerPort: 8081,
		HttpAPIPort:    8080,
		DefaultConfig: metricConfig{
			MetricName:  "manual",
			Active:      false,
			TargetSize:  1,
			MetricValue: 0,
		},
		ObjectMetrics: map[string]int64{},
	}
}

// parseConfig loads configuration from a YAML file if a path is provided.
// If path is empty, it falls back to the internal defaults. Missing or zero
// value fields are filled with defaults (best-effort merge).
func parseConfig(path string) (*config, error) {
	if path == "" {
		// No file provided: purely use defaults.
		return defaultConfig(), nil
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read: %w", err)
	}
	var fileCfg config
	if err = yaml.Unmarshal(fileBytes, &fileCfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	def := defaultConfig()
	// Apply defaults where necessary.
	if fileCfg.GrpcScalerPort == 0 {
		fileCfg.GrpcScalerPort = def.GrpcScalerPort
	}
	if fileCfg.HttpAPIPort == 0 {
		fileCfg.HttpAPIPort = def.HttpAPIPort
	}
	if fileCfg.DefaultConfig.MetricName == "" {
		fileCfg.DefaultConfig.MetricName = def.DefaultConfig.MetricName
	}
	if fileCfg.DefaultConfig.TargetSize == 0 { // targetSize 0 is likely invalid; treat as unset
		fileCfg.DefaultConfig.TargetSize = def.DefaultConfig.TargetSize
	}
	// Active false is a valid intentional state; no override.
	if fileCfg.ObjectMetrics == nil {
		fileCfg.ObjectMetrics = map[string]int64{}
	}
	return &fileCfg, nil
}
