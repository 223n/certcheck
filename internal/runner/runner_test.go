package runner_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/223n/certcheck/internal/checker"
	"github.com/223n/certcheck/internal/config"
	"github.com/223n/certcheck/internal/notify"
	"github.com/223n/certcheck/internal/runner"
)

// stubChecker returns canned results, and records what it was asked about.
type stubChecker struct {
	result checker.Result
	err    error
	calls  []stubCall
}

// stubCall is one call to [stubChecker.Check].
type stubCall struct {
	endpoint  string
	threshold int
}

func (s *stubChecker) Check(_ context.Context, endpoint string, thresholdDays int) (checker.Result, error) {
	s.calls = append(s.calls, stubCall{endpoint: endpoint, threshold: thresholdDays})
	if s.err != nil {
		return checker.Result{}, s.err
	}
	result := s.result
	result.Endpoint = endpoint
	return result, nil
}

// recordingNotifier records the messages it is asked to deliver.
type recordingNotifier struct {
	messages []notify.Message
	err      error
}

func (r *recordingNotifier) Notify(_ context.Context, msg notify.Message) error {
	r.messages = append(r.messages, msg)
	return r.err
}

// expiringResult is a result that crossed its threshold.
func expiringResult() checker.Result {
	return checker.Result{
		ExpiresAt: time.Date(2026, time.November, 2, 17, 39, 0, 0, checker.JST),
		DaysLeft:  5,
		Expiring:  true,
	}
}

// healthyResult is a result that is still far from expiry.
func healthyResult() checker.Result {
	return checker.Result{
		ExpiresAt: time.Date(2026, time.November, 2, 17, 39, 0, 0, checker.JST),
		DaysLeft:  61,
	}
}

// newRunner wires a runner around the given doubles and returns its log.
func newRunner(cfg *config.Config, c runner.Checker, n notify.Notifier) (*runner.Runner, *bytes.Buffer) {
	var logged bytes.Buffer
	logger := log.New(&logged, "", 0)
	return runner.New(cfg, c, n, logger), &logged
}

func TestRunNotifiesExpiringTarget(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{
			{Name: "example", Endpoint: "https://example.com", Threshold: 15},
		},
		Slack: config.Slack{
			HookURL:  "https://hooks.example.com/shared",
			Channel:  "general",
			Username: "certcheck",
			Icon:     ":lock:",
		},
	}
	check := &stubChecker{result: expiringResult()}
	notifier := &recordingNotifier{}

	r, logged := newRunner(cfg, check, notifier)
	r.Run(context.Background())

	if got, want := len(check.calls), 1; got != want {
		t.Fatalf("Check() calls = %d, want %d", got, want)
	}
	if got, want := (check.calls[0]), (stubCall{endpoint: "https://example.com", threshold: 15}); got != want {
		t.Errorf("Check() called with %+v, want %+v", got, want)
	}

	if got, want := len(notifier.messages), 1; got != want {
		t.Fatalf("Notify() calls = %d, want %d", got, want)
	}
	got := notifier.messages[0]
	want := notify.Message{
		WebhookURL: "https://hooks.example.com/shared",
		Channel:    "general",
		Username:   "certcheck",
		Icon:       ":lock:",
		Text:       "Cert Warning: https://example.com expire: 2026/11/02 17:39 at 5 days",
	}
	if got != want {
		t.Errorf("Notify() message = %+v, want %+v", got, want)
	}
	if !strings.Contains(logged.String(), want.Text) {
		t.Errorf("log = %q, want it to contain %q", logged.String(), want.Text)
	}
}

func TestRunDoesNotNotifyHealthyTarget(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{{Name: "example", Endpoint: "https://example.com", Threshold: 15}},
		Slack:   config.Slack{HookURL: "https://hooks.example.com/shared"},
	}
	notifier := &recordingNotifier{}

	r, logged := newRunner(cfg, &stubChecker{result: healthyResult()}, notifier)
	r.Run(context.Background())

	if len(notifier.messages) != 0 {
		t.Errorf("Notify() calls = %d, want 0", len(notifier.messages))
	}
	if want := "Cert OK: https://example.com expire: 2026/11/02 17:39 at 61 days"; !strings.Contains(logged.String(), want) {
		t.Errorf("log = %q, want it to contain %q", logged.String(), want)
	}
}

func TestRunAppliesPerTargetOverrides(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{{
			Name:      "example",
			Endpoint:  "https://example.com",
			Threshold: 15,
			HookURL:   "https://hooks.example.com/target",
			Channel:   "alerts",
			Username:  "certcheck(example)",
			Icon:      ":warning:",
		}},
		Slack: config.Slack{
			HookURL:  "https://hooks.example.com/shared",
			Channel:  "general",
			Username: "certcheck",
			Icon:     ":lock:",
		},
	}
	notifier := &recordingNotifier{}

	r, _ := newRunner(cfg, &stubChecker{result: expiringResult()}, notifier)
	r.Run(context.Background())

	if got, want := len(notifier.messages), 1; got != want {
		t.Fatalf("Notify() calls = %d, want %d", got, want)
	}
	got := notifier.messages[0]
	if got.WebhookURL != "https://hooks.example.com/target" {
		t.Errorf("WebhookURL = %q, want the target override", got.WebhookURL)
	}
	if got.Channel != "alerts" || got.Username != "certcheck(example)" || got.Icon != ":warning:" {
		t.Errorf("message = %+v, want the target overrides", got)
	}
}

func TestRunSkipsInvalidTarget(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{
			{Endpoint: "https://example.com"}, // no name
			{Name: "valid", Endpoint: "https://valid.example.com", Threshold: 7},
		},
	}
	check := &stubChecker{result: healthyResult()}
	notifier := &recordingNotifier{}

	r, logged := newRunner(cfg, check, notifier)
	r.Run(context.Background())

	if got, want := len(check.calls), 1; got != want {
		t.Fatalf("Check() calls = %d, want %d (the invalid target must be skipped)", got, want)
	}
	if got, want := check.calls[0].endpoint, "https://valid.example.com"; got != want {
		t.Errorf("Check() called with %q, want %q", got, want)
	}
	if !strings.Contains(logged.String(), "skipping target 0") {
		t.Errorf("log = %q, want it to report the skipped target", logged.String())
	}
}

func TestRunNotifiesFailedCheck(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{{Name: "example", Endpoint: "https://example.com", Threshold: 15}},
		Slack:   config.Slack{HookURL: "https://hooks.example.com/shared"},
	}
	notifier := &recordingNotifier{}

	r, logged := newRunner(cfg, &stubChecker{err: errors.New("connection refused")}, notifier)
	r.Run(context.Background())

	if got, want := len(notifier.messages), 1; got != want {
		t.Fatalf("Notify() calls = %d, want %d", got, want)
	}
	if got, want := notifier.messages[0].Text, "NG: connection refused"; got != want {
		t.Errorf("Notify() text = %q, want %q", got, want)
	}
	if !strings.Contains(logged.String(), "NG: connection refused") {
		t.Errorf("log = %q, want it to contain the failure", logged.String())
	}
}

func TestRunContinuesAfterNotifyFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Targets: []config.Target{
			{Name: "first", Endpoint: "https://first.example.com", Threshold: 15},
			{Name: "second", Endpoint: "https://second.example.com", Threshold: 15},
		},
		Slack: config.Slack{HookURL: "https://hooks.example.com/shared"},
	}
	check := &stubChecker{result: expiringResult()}
	notifier := &recordingNotifier{err: errors.New("slack is down")}

	r, logged := newRunner(cfg, check, notifier)
	r.Run(context.Background())

	if got, want := len(check.calls), 2; got != want {
		t.Fatalf("Check() calls = %d, want %d (a failed notification must not stop the run)", got, want)
	}
	if !strings.Contains(logged.String(), "slack is down") {
		t.Errorf("log = %q, want it to report the notification failure", logged.String())
	}
}

func TestRunWithoutTargets(t *testing.T) {
	t.Parallel()

	check := &stubChecker{}
	notifier := &recordingNotifier{}

	r, logged := newRunner(&config.Config{}, check, notifier)
	r.Run(context.Background())

	if len(check.calls) != 0 || len(notifier.messages) != 0 {
		t.Errorf("Check() calls = %d, Notify() calls = %d, want 0 and 0",
			len(check.calls), len(notifier.messages))
	}
	if logged.Len() != 0 {
		t.Errorf("log = %q, want it to be empty", logged.String())
	}
}

func TestRunLogsEveryValidationReasonOnOneLine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Targets: []config.Target{{Threshold: -1}}}

	r, logged := newRunner(cfg, &stubChecker{}, &recordingNotifier{})
	r.Run(context.Background())

	got := strings.TrimSpace(logged.String())
	if strings.Contains(got, "\n") {
		t.Errorf("log = %q, want a single line", got)
	}
	want := "skipping target 0: name is empty; endpoint is empty; threshold is less than 0"
	if got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}
