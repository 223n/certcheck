// Package config loads, parses and validates certcheck configuration files.
//
// A configuration file lists the endpoints to inspect (targets) together with
// the Slack settings used to notify about them. Every Slack setting may be
// overridden per target; see [Config.NotificationFor].
package config

import (
	"errors"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

// Config is the whole content of a certcheck configuration file.
type Config struct {
	// Targets lists the endpoints to inspect.
	Targets []Target `yaml:"targets"`
	// Slack holds the notification settings shared by every target.
	Slack Slack `yaml:"slack"`
}

// Target is a single endpoint whose certificate is inspected.
//
// The HookURL, Channel, Username and Icon fields override the corresponding
// [Slack] fields for this target only. An empty field means "inherit".
type Target struct {
	// Name is a human readable label for the target.
	Name string `yaml:"name"`
	// Endpoint is the URL to connect to, for example https://example.com.
	Endpoint string `yaml:"endpoint"`
	// Threshold is the number of remaining days at or below which the
	// certificate is reported as expiring.
	Threshold int `yaml:"threshold"`
	// HookURL overrides Slack.HookURL.
	HookURL string `yaml:"hook_url"`
	// Channel overrides Slack.Channel.
	Channel string `yaml:"channel"`
	// Username overrides Slack.Username.
	Username string `yaml:"username"`
	// Icon overrides Slack.Icon.
	Icon string `yaml:"icon"`
}

// Slack holds the settings of a Slack Incoming Webhook.
type Slack struct {
	// HookURL is the Incoming Webhook URL to post to.
	HookURL string `yaml:"hook_url"`
	// Channel is the channel to post in.
	Channel string `yaml:"channel"`
	// Username is the display name of the posting user.
	Username string `yaml:"username"`
	// Icon is the emoji shown next to the message, for example :lock:.
	Icon string `yaml:"icon"`
}

// Validation errors reported by [Target.Validate].
var (
	// ErrNameEmpty is reported when a target has no name.
	ErrNameEmpty = errors.New("name is empty")
	// ErrEndpointEmpty is reported when a target has no endpoint.
	ErrEndpointEmpty = errors.New("endpoint is empty")
	// ErrThresholdNegative is reported when a target has a negative threshold.
	ErrThresholdNegative = errors.New("threshold is less than 0")
)

// Load reads and parses the configuration file at path.
func Load(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	return Parse(buf)
}

// Parse parses the YAML document in data.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

// Validate reports why the target cannot be inspected, or nil when it can.
//
// All problems are reported at once, joined with [errors.Join], so a caller
// sees every reason a target was rejected rather than only the first one.
func (t Target) Validate() error {
	var errs []error
	if t.Name == "" {
		errs = append(errs, ErrNameEmpty)
	}
	if t.Endpoint == "" {
		errs = append(errs, ErrEndpointEmpty)
	}
	if t.Threshold < 0 {
		errs = append(errs, ErrThresholdNegative)
	}
	return errors.Join(errs...)
}

// NotificationFor resolves the Slack settings used to notify about t.
//
// It starts from the shared [Config.Slack] settings and replaces every field
// the target sets to a non-empty value.
func (c Config) NotificationFor(t Target) Slack {
	s := c.Slack
	if t.HookURL != "" {
		s.HookURL = t.HookURL
	}
	if t.Channel != "" {
		s.Channel = t.Channel
	}
	if t.Username != "" {
		s.Username = t.Username
	}
	if t.Icon != "" {
		s.Icon = t.Icon
	}
	return s
}
