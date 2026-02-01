package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Networks     []string `yaml:"networks"`
	ScanAllPorts bool     `yaml:"scan_all_ports"`
	Ports        []int    `yaml:"ports"`
	Schedule     string   `yaml:"schedule"`
	ScannerMode  string   `yaml:"scanner_mode"` // "zmap" or "tcp"
	Rate         int      `yaml:"rate"`
	Timeout      int      `yaml:"timeout"`
	Interface    string   `yaml:"interface"`
	APIURL       string   `yaml:"api_url"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		// Defaults
		Schedule: "*/15 * * * *",
		Rate:     10000,
		Timeout:  5,
		APIURL:   "http://127.0.0.1:8000",
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Default() *Config {
	return &Config{
		Networks:     []string{"192.168.1.0/24"},
		ScanAllPorts: false,
		Ports:        []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 445, 993, 995, 1433, 1521, 3306, 3389, 5432, 5900, 6379, 8080, 8443, 27017},
		Schedule:     "*/15 * * * *",
		ScannerMode:  "tcp",
		Rate:         100,
		Timeout:      5,
		APIURL:       "http://127.0.0.1:8000",
	}
}
