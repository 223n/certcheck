// Package checker inspects the TLS certificate presented by an endpoint.
//
// A [Checker] knows how to obtain a certificate and how long it remains valid.
// It deliberately knows nothing about configuration files or notifications, so
// it can be exercised against a real TLS server in tests.
package checker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultTimeout bounds a single endpoint check, including the TLS handshake.
const DefaultTimeout = 10 * time.Second

// TimeFormat is the layout expiry dates are rendered with.
const TimeFormat = "2006/01/02 15:04"

// JST is the fixed +09:00 zone expiry dates are reported in.
var JST = time.FixedZone("JST", 9*60*60)

// ErrNoTLS is returned when an endpoint answers without a TLS certificate,
// which is what happens for a plain http:// endpoint.
var ErrNoTLS = errors.New("endpoint did not present a TLS certificate")

// Result describes the certificate an endpoint presented.
type Result struct {
	// Endpoint is the URL that was inspected.
	Endpoint string
	// ExpiresAt is when the certificate stops being valid, in [JST].
	ExpiresAt time.Time
	// DaysLeft is the number of whole days until ExpiresAt.
	DaysLeft int
	// Expiring reports whether DaysLeft reached the target's threshold.
	Expiring bool
}

// String implements [fmt.Stringer] with the message certcheck logs and posts.
func (r Result) String() string {
	state := "OK"
	if r.Expiring {
		state = "Warning"
	}
	return fmt.Sprintf("Cert %s: %s expire: %s at %d days",
		state, r.Endpoint, r.ExpiresAt.Format(TimeFormat), r.DaysLeft)
}

// Checker inspects endpoints. Use [New] to build one.
type Checker struct {
	client *http.Client
	now    func() time.Time
	loc    *time.Location
}

// Option customises a [Checker].
type Option func(*Checker)

// WithHTTPClient replaces the HTTP client used to reach endpoints. Tests use
// it to trust a throwaway certificate authority.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Checker) { c.client = client }
}

// WithClock replaces the source of the current time, so that the remaining
// days of a certificate can be asserted deterministically.
func WithClock(now func() time.Time) Option {
	return func(c *Checker) { c.now = now }
}

// WithLocation replaces the time zone expiry dates are reported in.
func WithLocation(loc *time.Location) Option {
	return func(c *Checker) { c.loc = loc }
}

// New returns a Checker configured by opts.
func New(opts ...Option) *Checker {
	c := &Checker{
		client: &http.Client{Timeout: DefaultTimeout},
		now:    time.Now,
		loc:    JST,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Check connects to endpoint and reports the state of its certificate.
//
// thresholdDays is the number of remaining days at or below which the
// certificate counts as expiring.
func (c *Checker) Check(ctx context.Context, endpoint string, thresholdDays int) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request for %q: %w", endpoint, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("request %q: %w", endpoint, err)
	}
	defer resp.Body.Close()
	// The body is irrelevant; drain it so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return Result{}, fmt.Errorf("%q: %w", endpoint, ErrNoTLS)
	}

	now := c.now()
	expiresAt := resp.TLS.PeerCertificates[0].NotAfter.In(c.loc)
	return Result{
		Endpoint:  endpoint,
		ExpiresAt: expiresAt,
		DaysLeft:  int(expiresAt.Sub(now).Hours() / 24),
		Expiring:  !now.AddDate(0, 0, thresholdDays).Before(expiresAt),
	}, nil
}
