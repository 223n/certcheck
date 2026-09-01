package checker_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/223n/certcheck/internal/checker"
)

// now is the fixed instant every test reasons about. Certificates are issued
// relative to it, so the remaining days are deterministic.
var now = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

// newTLSServer starts an HTTPS server presenting a throwaway certificate that
// expires at notAfter, and returns a client that trusts it.
func newTLSServer(t *testing.T, notAfter time.Time) (*httptest.Server, *http.Client) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "certcheck test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}},
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	client := &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}
	return srv, client
}

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		daysUntilExpiry int
		threshold       int
		wantExpiring    bool
		wantDaysLeft    int
	}{
		"far from expiry": {
			daysUntilExpiry: 100,
			threshold:       15,
			wantExpiring:    false,
			wantDaysLeft:    100,
		},
		"inside the threshold": {
			daysUntilExpiry: 5,
			threshold:       15,
			wantExpiring:    true,
			wantDaysLeft:    5,
		},
		"exactly on the threshold": {
			daysUntilExpiry: 15,
			threshold:       15,
			wantExpiring:    true,
			wantDaysLeft:    15,
		},
		"one day past the threshold": {
			daysUntilExpiry: 16,
			threshold:       15,
			wantExpiring:    false,
			wantDaysLeft:    16,
		},
		"zero threshold only warns once expired": {
			daysUntilExpiry: 1,
			threshold:       0,
			wantExpiring:    false,
			wantDaysLeft:    1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			notAfter := now.AddDate(0, 0, tt.daysUntilExpiry)
			srv, client := newTLSServer(t, notAfter)

			c := checker.New(
				checker.WithHTTPClient(client),
				checker.WithClock(func() time.Time { return now }),
			)
			got, err := c.Check(context.Background(), srv.URL, tt.threshold)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			if got.Endpoint != srv.URL {
				t.Errorf("Endpoint = %q, want %q", got.Endpoint, srv.URL)
			}
			if got.Expiring != tt.wantExpiring {
				t.Errorf("Expiring = %t, want %t", got.Expiring, tt.wantExpiring)
			}
			if got.DaysLeft != tt.wantDaysLeft {
				t.Errorf("DaysLeft = %d, want %d", got.DaysLeft, tt.wantDaysLeft)
			}
			if !got.ExpiresAt.Equal(notAfter) {
				t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, notAfter)
			}
			if gotZone, _ := got.ExpiresAt.Zone(); gotZone != "JST" {
				t.Errorf("ExpiresAt zone = %q, want %q", gotZone, "JST")
			}
		})
	}
}

func TestCheckWithoutTLS(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// A plain http:// endpoint used to dereference a nil resp.TLS and panic.
	_, err := checker.New().Check(context.Background(), srv.URL, 15)
	if err == nil {
		t.Fatal("Check() error = nil, want an error for a plain HTTP endpoint")
	}
	if !errors.Is(err, checker.ErrNoTLS) {
		t.Errorf("Check() error = %v, want it to wrap ErrNoTLS", err)
	}
}

func TestCheckUnreachableEndpoint(t *testing.T) {
	t.Parallel()

	srv, client := newTLSServer(t, now.AddDate(0, 0, 30))
	url := srv.URL
	srv.Close()

	_, err := checker.New(checker.WithHTTPClient(client)).
		Check(context.Background(), url, 15)
	if err == nil {
		t.Fatal("Check() error = nil, want an error for a closed server")
	}
}

func TestCheckInvalidEndpoint(t *testing.T) {
	t.Parallel()

	_, err := checker.New().Check(context.Background(), "://not a url", 15)
	if err == nil {
		t.Fatal("Check() error = nil, want an error for a malformed URL")
	}
}

func TestCheckHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	srv, client := newTLSServer(t, now.AddDate(0, 0, 30))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := checker.New(checker.WithHTTPClient(client)).Check(ctx, srv.URL, 15)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check() error = %v, want it to wrap context.Canceled", err)
	}
}

func TestResultString(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, time.November, 2, 17, 39, 0, 0, checker.JST)

	tests := map[string]struct {
		result checker.Result
		want   string
	}{
		"ok": {
			result: checker.Result{
				Endpoint:  "https://example.com",
				ExpiresAt: expiresAt,
				DaysLeft:  61,
			},
			want: "Cert OK: https://example.com expire: 2026/11/02 17:39 at 61 days",
		},
		"warning": {
			result: checker.Result{
				Endpoint:  "https://example.com",
				ExpiresAt: expiresAt,
				DaysLeft:  5,
				Expiring:  true,
			},
			want: "Cert Warning: https://example.com expire: 2026/11/02 17:39 at 5 days",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tt.result.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckWithLocation(t *testing.T) {
	t.Parallel()

	notAfter := now.AddDate(0, 0, 30)
	srv, client := newTLSServer(t, notAfter)

	utc := time.FixedZone("UTC", 0)
	c := checker.New(
		checker.WithHTTPClient(client),
		checker.WithClock(func() time.Time { return now }),
		checker.WithLocation(utc),
	)
	got, err := c.Check(context.Background(), srv.URL, 15)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if gotZone, _ := got.ExpiresAt.Zone(); gotZone != "UTC" {
		t.Errorf("ExpiresAt zone = %q, want %q", gotZone, "UTC")
	}
	if !got.ExpiresAt.Equal(notAfter) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, notAfter)
	}
}
