package filter

import (
	_ "embed"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

//go:embed default_filters.yaml
var defaultFiltersYAML []byte

// DropLineRule defines a pattern to match lines for dropping.
type DropLineRule struct {
	Pattern  string `yaml:"pattern"`
	Category string `yaml:"category,omitempty"` // for logging, e.g. "npm-warn", "git-transfer"
}

// Config holds all filter configuration.
type Config struct {
	StripANSI             bool           `yaml:"strip_ansi"`
	CollapseBlankLines    int            `yaml:"collapse_blank_lines"`
	CollapseRepeatedLines int            `yaml:"collapse_repeated_lines"`
	CollapsePassingTests  int            `yaml:"collapse_passing_tests"`
	DropLines             []DropLineRule `yaml:"drop_lines"`
}

// FiltersFile is the top-level YAML structure.
type FiltersFile struct {
	Filters Config `yaml:"filters"`
}

// LoadConfig loads filter config from ~/.config/tego/filters.yaml if it exists,
// otherwise uses the embedded defaults.
func LoadConfig() (*Config, error) {
	// Check for user config
	home, err := os.UserHomeDir()
	if err == nil {
		userConfig := filepath.Join(home, ".config", "tego", "filters.yaml")
		if data, err := os.ReadFile(userConfig); err == nil {
			return parseConfig(data)
		}
	}

	return parseConfig(defaultFiltersYAML)
}

func parseConfig(data []byte) (*Config, error) {
	var f FiltersFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f.Filters, nil
}
