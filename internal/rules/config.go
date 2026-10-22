package rules

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  int               `yaml:"version"`
	Rules    map[string]string `yaml:"rules"`
	Exclude  []string          `yaml:"exclude"`
	Simulate SimulateConfig    `yaml:"simulate"`
}

type SimulateConfig struct {
	DefaultRef   string `yaml:"default-ref"`
	DefaultEvent string `yaml:"default-event"`
}

// LoadConfig attempts to load configuration from path or default locations (.gha-lint.yaml, .gha-lint.yml).
func LoadConfig(path string) (*Config, error) {
	configPath := path
	if configPath == "" {
		if _, err := os.Stat(".gha-lint.yaml"); err == nil {
			configPath = ".gha-lint.yaml"
		} else if _, err := os.Stat(".gha-lint.yml"); err == nil {
			configPath = ".gha-lint.yml"
		}
	}

	cfg := &Config{
		Version: 1,
		Rules:   make(map[string]string),
	}

	if configPath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if path != "" {
			return nil, err
		}
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Lowercase rule keys and actions
	normalizedRules := make(map[string]string, len(cfg.Rules))
	for k, v := range cfg.Rules {
		normalizedRules[strings.ToLower(k)] = strings.ToLower(v)
	}
	cfg.Rules = normalizedRules

	return cfg, nil
}

// FilterFindings applies config rules and exclusions to findings.
func FilterFindings(findings []Finding, cfg *Config, filePath string) []Finding {
	if cfg == nil {
		return findings
	}

	// Check file exclusions
	cleanPath := filepath.Clean(filePath)
	for _, exc := range cfg.Exclude {
		matched, _ := filepath.Match(exc, cleanPath)
		if matched || strings.Contains(cleanPath, strings.TrimSuffix(exc, "*")) {
			return nil
		}
	}

	var filtered []Finding
	for _, f := range findings {
		ruleAction, exists := cfg.Rules[strings.ToLower(f.Rule)]
		if exists {
			switch ruleAction {
			case "off", "ignore", "disabled":
				continue
			case "error":
				f.Severity = SeverityError
			case "warning", "warn":
				f.Severity = SeverityWarning
			case "info":
				f.Severity = SeverityInfo
			}
		}
		filtered = append(filtered, f)
	}

	return filtered
}
