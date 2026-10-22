package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	Teams              map[string]StringSlice       `yaml:"teams"`      // Project name to team allow-list; required only by scrum verbs (TEAM-01)
	Caps               Caps                         `yaml:"caps"`       // Request-shape caps (PLAT-04)
}

// Caps bounds the shape of a scrum request. A request over a cap is refused
// before the first call (PLAT-04), and the caps in force are disclosed in scope.
type Caps struct {
	MaxTeams         int `yaml:"max_teams" json:"max_teams"`
	MaxSprintHistory int `yaml:"max_sprint_history" json:"max_sprint_history"`
	MaxBacklogDepth  int `yaml:"max_backlog_depth" json:"max_backlog_depth"`
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
		Caps:           Caps{MaxTeams: 25, MaxSprintHistory: 12, MaxBacklogDepth: 500},
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
	// ADO_TEAMS="P/T,P/T" replaces the YAML map entirely, as ADO_PROJECTS does.
	// It splits on the first "/" only and trims both halves. An entry without a
	// "/" is kept as a project with a blank team, so Validate names it rather
	// than dropping it silently.
	if teams := os.Getenv("ADO_TEAMS"); teams != "" {
		c.Teams = map[string]StringSlice{}
		for _, e := range strings.Split(teams, ",") {
			if e = strings.TrimSpace(e); e == "" {
				continue
			}
			project, team, _ := strings.Cut(e, "/")
			project, team = strings.TrimSpace(project), strings.TrimSpace(team)
			c.Teams[project] = append(c.Teams[project], team)
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
	// TEAM-01 static checks. An absent teams map is valid here: only scrum
	// verbs require teams, and they check that before their first call. Keys are
	// walked in sorted order so the same config always reports the same error.
	keys := slices.Sorted(maps.Keys(c.Teams))
	for i, project := range keys {
		if !slices.ContainsFunc(c.Projects, func(p string) bool { return sameName(p, project) }) {
			return &ConfigError{Field: "teams", Message: fmt.Sprintf("project %q is not in projects", project)}
		}
		if slices.ContainsFunc(keys[:i], func(prev string) bool { return sameName(prev, project) }) {
			return &ConfigError{Field: "teams", Message: fmt.Sprintf("project %q is listed more than once", project)}
		}
		teams := c.Teams[project]
		for i, t := range teams {
			if strings.TrimSpace(t) == "" {
				return &ConfigError{Field: "teams", Message: fmt.Sprintf("team names in project %q must not be blank", project)}
			}
			if slices.ContainsFunc(teams[:i], func(prev string) bool { return sameName(prev, t) }) {
				return &ConfigError{Field: "teams", Message: fmt.Sprintf("duplicate team %q in project %q", t, project)}
			}
		}
	}
	return nil
}

// sameName is the one rule for matching configured project and team names:
// surrounding space is ignored and case is folded, as Azure DevOps treats
// names. Names are matched this way, never rewritten (D-11).
func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// ValidateScrum adds the checks only scrum verbs need (TEAM-01, PLAT-04): at
// least one team, a team list for every configured project, and positive caps.
// v1 verbs and the TUI never call it, so they keep working without teams. It
// runs after Validate and before any scrum call.
func (c *Config) ValidateScrum() error {
	if len(c.Teams) == 0 {
		return &ConfigError{Field: "teams", Message: "at least one team is required for scrum verbs"}
	}
	for _, p := range c.Projects {
		if len(c.TeamsFor(p)) == 0 {
			return &ConfigError{Field: "teams", Message: fmt.Sprintf("project %q has no teams; scrum verbs need at least one team per project", p)}
		}
	}
	if c.Caps.MaxTeams <= 0 || c.Caps.MaxSprintHistory <= 0 || c.Caps.MaxBacklogDepth <= 0 {
		return &ConfigError{Field: "caps", Message: "every cap must be greater than zero"}
	}
	return nil
}

// TeamsFor returns the configured teams for project, matching the teams key by
// sameName so a key written "p" serves project "P". Validate rejects two keys
// that match the same name, so at most one key matches.
func (c *Config) TeamsFor(project string) []string {
	for k, teams := range c.Teams {
		if sameName(k, project) {
			return teams
		}
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
//
// A fragment marker is refused even when the fragment is empty. So is any
// non-ASCII byte in the raw value or the host, because Go's case folding would
// map it onto a different ASCII org (CR-01), and so are the dot-segment orgs
// "." and "..". An org segment written percent-encoded, or one that
// url.PathEscape would change, is refused too: the SDK sends org_url's escaped
// form and the transport host rule compares that with the org's escaped form,
// so the org must be the same in both (08-REVIEW IN-01).
func parseOrgURL(raw string) (string, error) {
	// url.Parse leaves Fragment empty for a trailing "#", and the SDK would
	// then put every route into the fragment.
	if strings.Contains(raw, "#") || !isASCII(raw) {
		return "", errOrgURLShape
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errOrgURLShape
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errOrgURLShape
	}
	// A percent-encoded non-ASCII host decodes into the parsed host.
	if !isASCII(u.Hostname()) {
		return "", errOrgURLShape
	}
	host := strings.ToLower(u.Hostname())
	if host == "dev.azure.com" {
		segment, ok := strings.CutPrefix(u.Path, "/")
		segment = strings.TrimSuffix(segment, "/")
		if !ok || segment == "" || strings.Contains(segment, "/") || segment == "." || segment == ".." {
			return "", errOrgURLShape
		}
		// Both comparisons are needed: the escape of a backslash is %5C
		// itself, and a name such as a;b is unescaped yet PathEscape changes it.
		escaped, _ := strings.CutPrefix(u.EscapedPath(), "/")
		escaped = strings.TrimSuffix(escaped, "/")
		if escaped != segment || escaped != url.PathEscape(segment) {
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

// isASCII reports whether every byte of s is ASCII. It mirrors the transport's
// asciiLower check without importing internal/transport, which sits above
// config in the dependency order.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
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
