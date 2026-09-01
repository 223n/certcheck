package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/223n/certcheck/internal/notify"
)

// capturedPayload is the JSON certcheck sends in the payload form field.
type capturedPayload struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Icon     string `json:"icon_emoji"`
}

func TestSlackNotify(t *testing.T) {
	t.Parallel()

	var (
		gotMethod      string
		gotContentType string
		got            capturedPayload
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal([]byte(r.PostFormValue("payload")), &got); err != nil {
			t.Errorf("Unmarshal(payload) error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	msg := notify.Message{
		WebhookURL: srv.URL,
		Channel:    "alerts",
		Username:   "certcheck(example)",
		Icon:       ":warning:",
		Text:       "Cert Warning: https://example.com expire: 2026/11/02 17:39 at 5 days",
	}
	if err := notify.NewSlack().Notify(context.Background(), msg); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if want := "application/x-www-form-urlencoded"; gotContentType != want {
		t.Errorf("Content-Type = %q, want %q", gotContentType, want)
	}
	want := capturedPayload{
		Channel:  msg.Channel,
		Username: msg.Username,
		Text:     msg.Text,
		Icon:     msg.Icon,
	}
	if got != want {
		t.Errorf("payload = %+v, want %+v", got, want)
	}
}

func TestSlackNotifyWithoutWebhookURL(t *testing.T) {
	t.Parallel()

	err := notify.NewSlack().Notify(context.Background(), notify.Message{Text: "hello"})
	if !errors.Is(err, notify.ErrNoWebhookURL) {
		t.Fatalf("Notify() error = %v, want ErrNoWebhookURL", err)
	}
}

func TestSlackNotifyRejectsErrorStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid_payload", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	err := notify.NewSlack().Notify(context.Background(),
		notify.Message{WebhookURL: srv.URL, Text: "hello"})
	if err == nil {
		t.Fatal("Notify() error = nil, want an error for a 400 response")
	}
}

func TestSlackNotifyUnreachableWebhook(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	err := notify.NewSlack().Notify(context.Background(),
		notify.Message{WebhookURL: url, Text: "hello"})
	if err == nil {
		t.Fatal("Notify() error = nil, want an error for an unreachable webhook")
	}
}

func TestSlackNotifyHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := notify.NewSlack().Notify(ctx, notify.Message{WebhookURL: srv.URL, Text: "hello"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Notify() error = %v, want it to wrap context.Canceled", err)
	}
}

// TestSlackImplementsNotifier keeps the concrete type and the interface the
// runner depends on in sync.
func TestSlackImplementsNotifier(t *testing.T) {
	t.Parallel()

	var _ notify.Notifier = notify.NewSlack()
}

// roundTripperFunc lets a test act as the notifier's transport.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSlackNotifyWithHTTPClient(t *testing.T) {
	t.Parallel()

	var gotURL string
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Request:    r,
		}, nil
	})}

	msg := notify.Message{WebhookURL: "https://hooks.example.com/injected", Text: "hello"}
	if err := notify.NewSlack(notify.WithHTTPClient(client)).Notify(context.Background(), msg); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if gotURL != msg.WebhookURL {
		t.Errorf("request URL = %q, want %q", gotURL, msg.WebhookURL)
	}
}

func TestSlackNotifyMalformedWebhookURL(t *testing.T) {
	t.Parallel()

	// A control character makes the request impossible to build.
	err := notify.NewSlack().Notify(context.Background(),
		notify.Message{WebhookURL: "https://hooks.example.com/\x7f", Text: "hello"})
	if err == nil {
		t.Fatal("Notify() error = nil, want an error for a malformed webhook URL")
	}
}
