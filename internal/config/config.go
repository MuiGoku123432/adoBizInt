package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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
	PAT                string                       `yaml:"pat"`
	OrgURL             string                       `yaml:"org_url"`
	Projects           StringSlice                  `yaml:"projects"`
	Project            string                       `yaml:"project"`             // Backwards compat: single project
	Pipelines          StringSlice                  `yaml:"pipelines"`           // Optional: specific pipeline names to monitor
	ReleaseDefinitions StringSlice                  `yaml:"release_definitions"` // Optional: separate release definition names (falls back to pipelines)
	Repositories       StringSlice                  `yaml:"repositories"`        // Optional: specific repository names to filter PRs
	PollInterval       time.Duration                `yaml:"poll_interval"`
	RequestTimeout     time.Duration                `yaml:"request_timeout"` // Bounds a single HTTP call to Azure DevOps
	FetchTimeout       time.Duration                `yaml:"fetch_timeout"`   // Bounds a whole multi-project fan-out
	StateTransitions   map[string]map[string]string `yaml:"state_transitions"`
	UserEmail          string                       `yaml:"user_email"` // User's email for "Assigned to Me" filter
}

// LogValue, String and GoString render a Config without its PAT (WR-05). The
// credential sits in a plain exported field, so without them one
// `log.Info("Config loaded", "config", cfg)` or a debug Printf would write it
// to the log. They carry only what run() already logs, plus whether a PAT is
// set, never its value.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("org_url", c.OrgURL),
		slog.Any("projects", []string(c.Projects)),
		slog.Bool("pat_set", c.PAT != ""),
	)
}

func (c Config) String() string {
	return fmt.Sprintf("config{org_url: %s, projects: %v, pat_set: %t}", c.OrgURL, []string(c.Projects), c.PAT != "")
}

func (c Config) GoString() string { return c.String() }

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
	// Both timeout defaults are load-bearing, not cosmetic. A zero duration means
	// no bound at all for http.Client, and instant expiry for context.WithTimeout.
	cfg := &Config{
		PollInterval:   30 * time.Second,
		RequestTimeout: 30 * time.Second,
		FetchTimeout:   2 * time.Minute,
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
	// D-12: https plus one of the two cloud shapes makes the {org} the transport
	// host rule binds the PAT to unambiguous, and keeps the PAT off cleartext.
	// On-prem Azure DevOps Server is no longer supported. The value is checked,
	// never rewritten.
	if _, err := parseOrgURL(c.OrgURL); err != nil {
		return &ConfigError{Field: "org_url", Message: err.Error()}
	}
	// D-10: an empty list makes every read fan out over nothing and report
	// complete:true. This covers the CLI and the TUI alike, because run()
	// validates before both.
	if len(c.Projects) == 0 {
		return &ConfigError{Field: "projects", Message: "at least one project is required"}
	}
	// D-11: a blank name would reach the read as a project with no name. Names
	// are validated, never trimmed or rewritten, so what the operator configured
	// is what the read uses.
	for _, p := range c.Projects {
		if strings.TrimSpace(p) == "" {
			return &ConfigError{Field: "projects", Message: "project names must not be blank"}
		}
	}
	// A fan-out budget at or below the poll interval cancels itself exactly as the
	// next tick fires, producing a view that fails permanently and looks like an outage.
	if c.FetchTimeout <= c.PollInterval {
		return &ConfigError{Field: "fetch_timeout", Message: "must be greater than poll_interval"}
	}
	return nil
}

// errOrgURLShape is the one message for every org_url that is not a cloud
// organization URL (D-12).
var errOrgURLShape = errors.New("org_url must be https://dev.azure.com/{org} or https://{org}.visualstudio.com")

// parseOrgURL returns the lowercase {org} that org_url names (D-12). It admits
// exactly two cloud shapes: https://dev.azure.com/{org}, with one non-empty
// path segment and an optional trailing slash, and https://{org}.visualstudio.com,
// with an empty or "/" path and a host label that contains no dot. Anything
// else, including userinfo, a port, a query or a fragment, is refused. The
// parse is structural rather than a character class, so an unusual but valid
// org name is not rejected (RESEARCH A7).
func parseOrgURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errOrgURLShape
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errOrgURLShape
	}
	host := strings.ToLower(u.Hostname())
	if host == "dev.azure.com" {
		segment, ok := strings.CutPrefix(u.Path, "/")
		segment = strings.TrimSuffix(segment, "/")
		if !ok || segment == "" || strings.Contains(segment, "/") {
			return "", errOrgURLShape
		}
		return strings.ToLower(segment), nil
	}
	if label, ok := strings.CutSuffix(host, ".visualstudio.com"); ok {
		if label == "" || strings.Contains(label, ".") || (u.Path != "" && u.Path != "/") {
			return "", errOrgURLShape
		}
		return label, nil
	}
	return "", errOrgURLShape
}

// Org returns the lowercase organization name parsed from OrgURL, or "" when
// OrgURL does not parse. Callers run after Validate. D-09 binds the PAT to this
// value: the transport sends the credential only to hosts and paths that belong
// to it.
func (c *Config) Org() string {
	org, err := parseOrgURL(c.OrgURL)
	if err != nil {
		return ""
	}
	return org
}

type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
