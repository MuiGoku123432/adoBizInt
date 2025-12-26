package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// StringSlice is a custom type that can unmarshal from either a YAML array or comma-separated string
type StringSlice []string

func (s *StringSlice) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		// YAML array: [a, b, c] or multi-line list
		var arr []string
		if err := value.Decode(&arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	// Scalar string: "a,b,c"
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}
	for _, p := range strings.Split(str, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*s = append(*s, p)
		}
	}
	return nil
}

type Config struct {
	PAT              string                       `yaml:"pat"`
	OrgURL           string                       `yaml:"org_url"`
	Projects         StringSlice                  `yaml:"projects"`
	Project          string                       `yaml:"project"`      // Backwards compat: single project
	Pipelines        StringSlice                  `yaml:"pipelines"`    // Optional: specific pipeline names to monitor
	Repositories     StringSlice                  `yaml:"repositories"` // Optional: specific repository names to filter PRs
	PollInterval     time.Duration                `yaml:"poll_interval"`
	StateTransitions map[string]map[string]string `yaml:"state_transitions"`
	UserEmail        string                       `yaml:"user_email"` // User's email for "Assigned to Me" filter
}

// DefaultStateTransitions provides default state transitions for common work item types
// Flow: New -> Active -> Resolved -> Closed
var DefaultStateTransitions = map[string]map[string]string{
	"User Story": {
		"New":      "Active",
		"Active":   "Resolved",
		"Resolved": "Closed",
	},
	"Task": {
		"New":    "Active",
		"Active": "Closed",
	},
	"Bug": {
		"New":      "Active",
		"Active":   "Resolved",
		"Resolved": "Closed",
	},
	"Feature": {
		"New":      "Active",
		"Active":   "Resolved",
		"Resolved": "Closed",
	},
	"Epic": {
		"New":      "Active",
		"Active":   "Resolved",
		"Resolved": "Closed",
	},
}

func Load() (*Config, error) {
	cfg := &Config{
		PollInterval: 30 * time.Second,
	}

	if err := cfg.loadFromFile(); err != nil {
		// File not found is ok, we'll use env vars
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	cfg.loadFromEnv()

	// Backwards compat: if single Project is set and Projects is empty, use it
	if len(cfg.Projects) == 0 && cfg.Project != "" {
		cfg.Projects = []string{cfg.Project}
	}

	// Apply default state transitions if not configured
	if cfg.StateTransitions == nil {
		cfg.StateTransitions = DefaultStateTransitions
	}

	return cfg, nil
}

func (c *Config) loadFromFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(home, ".config", "adoBizInt", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, c)
}

func (c *Config) loadFromEnv() {
	if pat := os.Getenv("ADO_PAT"); pat != "" {
		c.PAT = pat
	}
	if orgURL := os.Getenv("ADO_ORG_URL"); orgURL != "" {
		c.OrgURL = orgURL
	}
	if projects := os.Getenv("ADO_PROJECTS"); projects != "" {
		// Parse comma-separated projects from env var
		c.Projects = nil // Clear any existing
		for _, p := range strings.Split(projects, ",") {
			if p = strings.TrimSpace(p); p != "" {
				c.Projects = append(c.Projects, p)
			}
		}
	}
	// Backwards compat
	if project := os.Getenv("ADO_PROJECT"); project != "" {
		c.Project = project
	}
}

func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	return nil
}

type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
