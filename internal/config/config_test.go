package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/223n/certcheck/internal/config"
)

func TestParse(t *testing.T) {
	t.Parallel()

	const document = `
targets:
  - name: google
    endpoint: https://google.com
    threshold: 15
    channel: random
    username: certcheck(google)
slack:
  hook_url: https://hooks.slack.com/services/a/b/c
  channel: general
  username: certcheck
  icon: ":lock:"
`

	cfg, err := config.Parse([]byte(document))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got, want := len(cfg.Targets), 1; got != want {
		t.Fatalf("len(Targets) = %d, want %d", got, want)
	}
	target := cfg.Targets[0]
	if got, want := target.Name, "google"; got != want {
		t.Errorf("Targets[0].Name = %q, want %q", got, want)
	}
	if got, want := target.Endpoint, "https://google.com"; got != want {
		t.Errorf("Targets[0].Endpoint = %q, want %q", got, want)
	}
	if got, want := target.Threshold, 15; got != want {
		t.Errorf("Targets[0].Threshold = %d, want %d", got, want)
	}
	if got, want := target.Channel, "random"; got != want {
		t.Errorf("Targets[0].Channel = %q, want %q", got, want)
	}
	if got, want := cfg.Slack.HookURL, "https://hooks.slack.com/services/a/b/c"; got != want {
		t.Errorf("Slack.HookURL = %q, want %q", got, want)
	}
	if got, want := cfg.Slack.Icon, ":lock:"; got != want {
		t.Errorf("Slack.Icon = %q, want %q", got, want)
	}
}

func TestParseInvalidYAML(t *testing.T) {
	t.Parallel()

	if _, err := config.Parse([]byte("targets: [")); err == nil {
		t.Fatal("Parse() error = nil, want an error for a malformed document")
	}
}

func TestParseEmptyDocument(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse(nil)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(cfg.Targets) != 0 {
		t.Errorf("len(Targets) = %d, want 0", len(cfg.Targets))
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "certcheck.yml")
	document := "targets:\n  - name: example\n    endpoint: https://example.com\n    threshold: 7\n"
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := len(cfg.Targets), 1; got != want {
		t.Fatalf("len(Targets) = %d, want %d", got, want)
	}
	if got, want := cfg.Targets[0].Endpoint, "https://example.com"; got != want {
		t.Errorf("Targets[0].Endpoint = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := config.Load(filepath.Join(t.TempDir(), "absent.yml"))
	if err == nil {
		t.Fatal("Load() error = nil, want an error for a missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load() error = %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestTargetValidate(t *testing.T) {
	t.Parallel()

	valid := config.Target{Name: "example", Endpoint: "https://example.com", Threshold: 7}

	tests := map[string]struct {
		target config.Target
		want   []error
	}{
		"valid": {
			target: valid,
		},
		"zero threshold is allowed": {
			target: config.Target{Name: "example", Endpoint: "https://example.com"},
		},
		"missing name": {
			target: config.Target{Endpoint: "https://example.com"},
			want:   []error{config.ErrNameEmpty},
		},
		"missing endpoint": {
			target: config.Target{Name: "example"},
			want:   []error{config.ErrEndpointEmpty},
		},
		"negative threshold": {
			target: config.Target{Name: "example", Endpoint: "https://example.com", Threshold: -1},
			want:   []error{config.ErrThresholdNegative},
		},
		"every problem is reported at once": {
			target: config.Target{Threshold: -1},
			want: []error{
				config.ErrNameEmpty,
				config.ErrEndpointEmpty,
				config.ErrThresholdNegative,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tt.target.Validate()
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %v", tt.want)
			}
			for _, want := range tt.want {
				if !errors.Is(err, want) {
					t.Errorf("Validate() error = %v, want it to wrap %v", err, want)
				}
			}
		})
	}
}

func TestConfigNotificationFor(t *testing.T) {
	t.Parallel()

	shared := config.Slack{
		HookURL:  "https://hooks.example.com/shared",
		Channel:  "general",
		Username: "certcheck",
		Icon:     ":lock:",
	}

	tests := map[string]struct {
		target config.Target
		want   config.Slack
	}{
		"inherits every shared setting": {
			target: config.Target{Name: "example"},
			want:   shared,
		},
		"overrides one setting": {
			target: config.Target{Name: "example", Channel: "random"},
			want: config.Slack{
				HookURL:  shared.HookURL,
				Channel:  "random",
				Username: shared.Username,
				Icon:     shared.Icon,
			},
		},
		"overrides every setting": {
			target: config.Target{
				Name:     "example",
				HookURL:  "https://hooks.example.com/target",
				Channel:  "alerts",
				Username: "certcheck(example)",
				Icon:     ":warning:",
			},
			want: config.Slack{
				HookURL:  "https://hooks.example.com/target",
				Channel:  "alerts",
				Username: "certcheck(example)",
				Icon:     ":warning:",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{Slack: shared}
			if got := cfg.NotificationFor(tt.target); got != tt.want {
				t.Errorf("NotificationFor() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConfigNotificationForDoesNotMutateShared(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Slack: config.Slack{Channel: "general"}}
	_ = cfg.NotificationFor(config.Target{Channel: "random"})

	if got, want := cfg.Slack.Channel, "general"; got != want {
		t.Errorf("Slack.Channel = %q, want %q", got, want)
	}
}
