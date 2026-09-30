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
