package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout bounds a single delivery attempt.
const DefaultTimeout = 10 * time.Second

// ErrNoWebhookURL is returned when a message carries no webhook URL, which
// means neither the shared Slack settings nor the target configured one.
var ErrNoWebhookURL = errors.New("webhook url is empty")

// slackPayload mirrors the JSON accepted by the Incoming Webhook API.
type slackPayload struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Icon     string `json:"icon_emoji"`
}

// Slack posts messages to a Slack Incoming Webhook. It implements [Notifier].
type Slack struct {
	client *http.Client
}

// SlackOption customises a [Slack].
type SlackOption func(*Slack)

// WithHTTPClient replaces the HTTP client used to post messages.
func WithHTTPClient(client *http.Client) SlackOption {
	return func(s *Slack) { s.client = client }
}

// NewSlack returns a Slack notifier configured by opts.
func NewSlack(opts ...SlackOption) *Slack {
	s := &Slack{client: &http.Client{Timeout: DefaultTimeout}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Notify posts msg to its webhook URL.
//
// A delivery failure is returned rather than fatal: one unreachable webhook
// must not stop the remaining targets from being checked.
func (s *Slack) Notify(ctx context.Context, msg Message) error {
	if msg.WebhookURL == "" {
		return ErrNoWebhookURL
	}

	payload, err := json.Marshal(slackPayload{
		Channel:  msg.Channel,
		Username: msg.Username,
		Text:     msg.Text,
		Icon:     msg.Icon,
	})
	if err != nil {
		return fmt.Errorf("encode slack payload: %w", err)
	}

	form := url.Values{"payload": {string(payload)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msg.WebhookURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("slack responded with %s", resp.Status)
	}
	return nil
}
