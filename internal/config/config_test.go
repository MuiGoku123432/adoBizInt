package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeConfig isolates Load() from the developer's real config file. Load()
// reads $HOME/.config/adoBizInt/config.yaml unconditionally and has no injection
// seam; redirecting HOME is cheaper than adding one this package does not
// otherwise need.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if body == "" {
		return
	}
	dir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}
}

func TestLoadAppliesTimeoutDefaults(t *testing.T) {
	t.Run("absent config file", func(t *testing.T) {
		writeConfig(t, "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.RequestTimeout == 0 {
			t.Errorf("Load() RequestTimeout = %v, want non-zero — a zero duration means no bound at all for http.Client", cfg.RequestTimeout)
		}
		if cfg.FetchTimeout == 0 {
			t.Errorf("Load() FetchTimeout = %v, want non-zero — a zero duration means instant expiry for context.WithTimeout", cfg.FetchTimeout)
		}
	})

	t.Run("config file setting only poll_interval", func(t *testing.T) {
		writeConfig(t, "poll_interval: 15s\n")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.PollInterval != 15*time.Second {
			t.Errorf("Load() PollInterval = %v, want 15s", cfg.PollInterval)
		}
		if cfg.RequestTimeout == 0 {
			t.Errorf("Load() RequestTimeout = %v, want non-zero", cfg.RequestTimeout)
		}
		if cfg.FetchTimeout == 0 {
			t.Errorf("Load() FetchTimeout = %v, want non-zero", cfg.FetchTimeout)
		}
	})

	t.Run("fetch budget default exceeds poll interval default", func(t *testing.T) {
		writeConfig(t, "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.FetchTimeout <= cfg.PollInterval {
			t.Errorf("Load() FetchTimeout = %v, PollInterval = %v, want the defaults to satisfy Validate()", cfg.FetchTimeout, cfg.PollInterval)
		}
	})
}

func TestLoadParsesTimeoutDurationsFromYAML(t *testing.T) {
	writeConfig(t, "request_timeout: 45s\nfetch_timeout: 3m\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.RequestTimeout != 45*time.Second {
		t.Errorf("Load() RequestTimeout = %v, want 45s", cfg.RequestTimeout)
	}
	if cfg.FetchTimeout != 3*time.Minute {
		t.Errorf("Load() FetchTimeout = %v, want 3m", cfg.FetchTimeout)
	}
}

func TestValidate(t *testing.T) {
	base := func() *Config {
		return &Config{
			PAT:          "token",
			OrgURL:       "https://dev.azure.com/example",
			PollInterval: 30 * time.Second,
			FetchTimeout: 2 * time.Minute,
			Projects:     StringSlice{"P"},
		}
	}

	tests := []struct {
		name      string
		mutate    func(*Config)
		wantField string
	}{
		{
			name:      "valid config",
			mutate:    func(*Config) {},
			wantField: "",
		},
		{
			name:      "missing pat",
			mutate:    func(c *Config) { c.PAT = "" },
			wantField: "pat",
		},
		{
			name:      "missing org url",
			mutate:    func(c *Config) { c.OrgURL = "" },
			wantField: "org_url",
		},
		{
			// A fan-out slower than the poll interval is cancelled by its own
			// deadline exactly as the next tick fires.
			name:      "fetch timeout equal to poll interval",
			mutate:    func(c *Config) { c.FetchTimeout = c.PollInterval },
			wantField: "fetch_timeout",
		},
		{
			name:      "fetch timeout below poll interval",
			mutate:    func(c *Config) { c.FetchTimeout = c.PollInterval - time.Second },
			wantField: "fetch_timeout",
		},
		{
			// D-10: an empty list makes every read report complete:true over nothing.
			name:      "nil projects",
			mutate:    func(c *Config) { c.Projects = nil },
			wantField: "projects",
		},
		{
			name:      "empty projects slice",
			mutate:    func(c *Config) { c.Projects = StringSlice{} },
			wantField: "projects",
		},
		{
			// The projects check runs after pat, so a missing credential is still
			// the first thing the operator is told about.
			name:      "missing pat with no projects",
			mutate:    func(c *Config) { c.PAT = ""; c.Projects = nil },
			wantField: "pat",
		},
		{
			// D-11: a blank name would reach the read as a project with no name.
			name:      "blank project entry",
			mutate:    func(c *Config) { c.Projects = StringSlice{""} },
			wantField: "projects",
		},
		{
			name:      "whitespace project entry",
			mutate:    func(c *Config) { c.Projects = StringSlice{" \t "} },
			wantField: "projects",
		},
		{
			name:      "non-breaking-space project entry",
			mutate:    func(c *Config) { c.Projects = StringSlice{" "} },
			wantField: "projects",
		},
		{
			name:      "blank entry beside a valid one",
			mutate:    func(c *Config) { c.Projects = StringSlice{"A", " "} },
			wantField: "projects",
		},
		{
			name:      "interior space is a valid name",
			mutate:    func(c *Config) { c.Projects = StringSlice{"My Project"} },
			wantField: "",
		},
		{
			// Names are validated, never rewritten, so padding is accepted as written.
			name:      "padded name is accepted as written",
			mutate:    func(c *Config) { c.Projects = StringSlice{" P "} },
			wantField: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(cfg)

			err := cfg.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			cfgErr, ok := err.(*ConfigError)
			if !ok {
				t.Fatalf("Validate() error = %v (%T), want *ConfigError", err, err)
			}
			if cfgErr.Field != tt.wantField {
				t.Errorf("Validate() ConfigError.Field = %v, want %v", cfgErr.Field, tt.wantField)
			}
		})
	}
}

// TestValidateOrgURL pins D-12: org_url must be https and one of the two cloud
// shapes, https://dev.azure.com/{org} or https://{org}.visualstudio.com, each
// with an optional trailing slash. Anything else, on-prem Azure DevOps Server
// included, is a named org_url error, so run() exits 3 before any request.
// The check runs after the missing-PAT check, so a missing credential is still
// reported first.
func TestValidateOrgURL(t *testing.T) {
	base := func() *Config {
		return &Config{
			PAT:          "token",
			OrgURL:       "https://dev.azure.com/example",
			PollInterval: 30 * time.Second,
			FetchTimeout: 2 * time.Minute,
			Projects:     StringSlice{"P"},
		}
	}

	orgURL := func(u string) func(*Config) { return func(c *Config) { c.OrgURL = u } }
	tests := []struct {
		name      string
		mutate    func(*Config)
		wantField string
	}{
		{name: "dev.azure.com org", mutate: orgURL("https://dev.azure.com/example"), wantField: ""},
		{name: "dev.azure.com org with trailing slash", mutate: orgURL("https://dev.azure.com/example/"), wantField: ""},
		{name: "dev.azure.com mixed case", mutate: orgURL("https://Dev.Azure.com/Example"), wantField: ""},
		{name: "visualstudio.com org", mutate: orgURL("https://example.visualstudio.com"), wantField: ""},
		{name: "visualstudio.com org with trailing slash", mutate: orgURL("https://example.visualstudio.com/"), wantField: ""},

		{name: "http scheme", mutate: orgURL("http://dev.azure.com/example"), wantField: "org_url"},
		{name: "userinfo", mutate: orgURL("https://user@dev.azure.com/example"), wantField: "org_url"},
		{name: "explicit port", mutate: orgURL("https://dev.azure.com:443/example"), wantField: "org_url"},
		{name: "query", mutate: orgURL("https://dev.azure.com/example?x=1"), wantField: "org_url"},
		{name: "fragment", mutate: orgURL("https://dev.azure.com/example#f"), wantField: "org_url"},
		{name: "org and project path", mutate: orgURL("https://dev.azure.com/example/project"), wantField: "org_url"},
		{name: "dev.azure.com root with slash", mutate: orgURL("https://dev.azure.com/"), wantField: "org_url"},
		{name: "dev.azure.com root", mutate: orgURL("https://dev.azure.com"), wantField: "org_url"},
		{name: "empty first path segment", mutate: orgURL("https://dev.azure.com//example"), wantField: "org_url"},
		{name: "foreign host", mutate: orgURL("https://example.com/org"), wantField: "org_url"},
		{name: "visualstudio.com with a path", mutate: orgURL("https://example.visualstudio.com/path"), wantField: "org_url"},
		{name: "visualstudio.com two labels", mutate: orgURL("https://a.b.visualstudio.com"), wantField: "org_url"},
		{name: "bare visualstudio.com", mutate: orgURL("https://visualstudio.com"), wantField: "org_url"},
		{name: "on-prem server collection", mutate: orgURL("https://tfs.corp.local/tfs/DefaultCollection"), wantField: "org_url"},

		{
			// The shape check runs after pat, so a missing credential is still
			// the first thing the operator is told about.
			name:      "missing pat with a bad org url",
			mutate:    func(c *Config) { c.PAT = ""; c.OrgURL = "http://dev.azure.com/example" },
			wantField: "pat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(cfg)

			err := cfg.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			cfgErr, ok := err.(*ConfigError)
			if !ok {
				t.Fatalf("Validate() error = %v (%T), want *ConfigError", err, err)
			}
			if cfgErr.Field != tt.wantField {
				t.Errorf("Validate() ConfigError.Field = %v, want %v", cfgErr.Field, tt.wantField)
			}
		})
	}
}

// TestOrg pins the {org} the transport binds the PAT to: lowercase, from either
// cloud shape, and empty for a value that does not parse, so the transport
// refuses every request rather than binding to a guess.
func TestOrg(t *testing.T) {
	tests := []struct {
		orgURL string
		want   string
	}{
		{orgURL: "https://dev.azure.com/Fabrikam/", want: "fabrikam"},
		{orgURL: "https://Fabrikam.visualstudio.com", want: "fabrikam"},
		{orgURL: "https://[::1", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.orgURL, func(t *testing.T) {
			cfg := &Config{OrgURL: tt.orgURL}
			if got := cfg.Org(); got != tt.want {
				t.Errorf("Org() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLoadRejectsBlankProjects drives every load path that can yield an empty
// or blank projects list through Load and then Validate. The YAML sequence and
// the legacy project key do not trim, so D-11's blank check is the only guard on
// those paths. The ADO_PROJECTS and YAML scalar paths already trim and drop
// empty parts, which turns their blanks into an empty list that D-10 rejects.
func TestLoadRejectsBlankProjects(t *testing.T) {
	const creds = "pat: fixture-pat\norg_url: https://dev.azure.com/fixture\n"

	tests := []struct {
		name      string
		yaml      string
		envList   string
		wantValid bool
	}{
		{name: "yaml sequence with an empty string", yaml: `projects: [""]`},
		{name: "yaml sequence with spaces", yaml: `projects: ["  "]`},
		{name: "yaml sequence with a blank beside a valid name", yaml: `projects: ["A", " "]`},
		{name: "legacy project key of spaces", yaml: `project: "  "`},
		{name: "yaml scalar of spaces", yaml: `projects: "  "`},
		{name: "env list of separators only", envList: " , "},
		{name: "no projects anywhere"},
		{name: "valid yaml sequence", yaml: `projects: ["A", "B"]`, wantValid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeConfig(t, creds+tt.yaml+"\n")
			t.Setenv("ADO_PAT", "")
			t.Setenv("ADO_ORG_URL", "")
			t.Setenv("ADO_PROJECT", "")
			t.Setenv("ADO_PROJECTS", tt.envList)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			err = cfg.Validate()
			if tt.wantValid {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			cfgErr, ok := err.(*ConfigError)
			if !ok {
				t.Fatalf("Validate() error = %v (%T), want *ConfigError; Projects = %q", err, err, cfg.Projects)
			}
			if cfgErr.Field != "projects" {
				t.Errorf("Validate() ConfigError.Field = %v, want projects", cfgErr.Field)
			}
		})
	}
}

// TestLoadedConfigNeverRendersPAT pins WR-05 where the secret actually lives.
// A loaded Config holds the PAT, so logging it as one slog attribute or
// formatting it with fmt, the most likely careless edit to run()'s "Config
// loaded" line, must not print the credential. The org URL must still appear,
// or the check passes because nothing was rendered.
func TestLoadedConfigNeverRendersPAT(t *testing.T) {
	const pat = "pat-fixture-do-not-disclose-5e7b20"
	writeConfig(t, "")
	t.Setenv("ADO_PAT", pat)
	t.Setenv("ADO_ORG_URL", "https://dev.azure.com/fabrikam")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.PAT != pat {
		t.Fatalf("Load() PAT not taken from ADO_PAT; the check below would be vacuous")
	}

	var logged bytes.Buffer
	text := slog.New(slog.NewTextHandler(&logged, nil))
	jsonLog := slog.New(slog.NewJSONHandler(&logged, nil))
	for _, l := range []*slog.Logger{text, jsonLog} {
		l.Info("Config loaded", "config", cfg)
		l.Info("Config loaded", "config", *cfg)
	}

	rendered := map[string]string{"slog": logged.String()}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		rendered["fmt "+verb+" pointer"] = fmt.Sprintf(verb, cfg)
		rendered["fmt "+verb+" value"] = fmt.Sprintf(verb, *cfg)
	}

	for how, out := range rendered {
		if strings.Contains(out, pat) {
			t.Errorf("%s renders the PAT: %s", how, out)
		}
		if !strings.Contains(out, "fabrikam") {
			t.Errorf("%s does not render the org URL, so the check is vacuous: %s", how, out)
		}
	}
}
