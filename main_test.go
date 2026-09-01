package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newTestLogger returns a logger writing into the returned buffer.
func newTestLogger() (*log.Logger, *bytes.Buffer) {
	var logged bytes.Buffer
	return log.New(&logged, "", 0), &logged
}

func TestVersion(t *testing.T) {
	t.Parallel()

	got := version()
	for _, want := range []string{Version, Revision, runtime.GOOS, runtime.GOARCH, "build"} {
		if !strings.Contains(got, want) {
			t.Errorf("version() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRunPrintsVersion(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			logger, logged := newTestLogger()
			if err := run([]string{flag}, logger); err != nil {
				t.Fatalf("run(%q) error = %v", flag, err)
			}
			if got, want := strings.TrimSpace(logged.String()), version(); got != want {
				t.Errorf("run(%q) logged %q, want %q", flag, got, want)
			}
		})
	}
}

func TestRunMissingConfig(t *testing.T) {
	t.Parallel()

	logger, _ := newTestLogger()
	err := run([]string{"-c", filepath.Join(t.TempDir(), "absent.yml")}, logger)
	if err == nil {
		t.Fatal("run() error = nil, want an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "absent.yml") {
		t.Errorf("run() error = %v, want it to name the missing file", err)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	t.Parallel()

	logger, _ := newTestLogger()
	// flag.ContinueOnError writes its own usage message; keep it out of the
	// test output.
	if err := run([]string{"-nope"}, logger); err == nil {
		t.Fatal("run() error = nil, want an error for an unknown flag")
	}
}

// TestRunReportsUnreachableTarget exercises the whole wiring: a configuration
// file is loaded, its single target is checked, and the failure is reported
// without stopping the process.
func TestRunReportsUnreachableTarget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "certcheck.yml")
	document := strings.Join([]string{
		"targets:",
		"  - name: unreachable",
		"    endpoint: https://127.0.0.1:1",
		"    threshold: 15",
	}, "\n")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	logger, logged := newTestLogger()
	if err := run([]string{"-c", path}, logger); err != nil {
		t.Fatalf("run() error = %v, want a single unreachable target to be reported, not fatal", err)
	}
	if !strings.Contains(logged.String(), "NG:") {
		t.Errorf("log = %q, want it to report the unreachable target", logged.String())
	}
}

// TestRunSkipsInvalidTarget checks that a malformed target does not abort the
// process either.
func TestRunSkipsInvalidTarget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "certcheck.yml")
	if err := os.WriteFile(path, []byte("targets:\n  - threshold: -1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	logger, logged := newTestLogger()
	if err := run([]string{"-c", path}, logger); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
	if !strings.Contains(logged.String(), "skipping target 0") {
		t.Errorf("log = %q, want it to report the skipped target", logged.String())
	}
}
