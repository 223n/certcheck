// Package runner orchestrates the certificate check of every configured
// target.
//
// It is the only place that knows the order of operations: validate a target,
// inspect its certificate, decide whether to notify, and report the outcome.
// It depends on the [Checker] and [notify.Notifier] interfaces, so both can be
// replaced without changing this package.
package runner

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/223n/certcheck/internal/checker"
	"github.com/223n/certcheck/internal/config"
	"github.com/223n/certcheck/internal/notify"
)

// Checker inspects the certificate of a single endpoint.
//
// The interface is declared here, next to its consumer, so that the runner
// depends on the behaviour it needs rather than on a concrete implementation.
type Checker interface {
	Check(ctx context.Context, endpoint string, thresholdDays int) (checker.Result, error)
}

// Runner checks every target of a configuration. Use [New] to build one.
type Runner struct {
	cfg      *config.Config
	checker  Checker
	notifier notify.Notifier
	logger   *log.Logger
}

// New returns a Runner that checks the targets of cfg.
func New(cfg *config.Config, c Checker, n notify.Notifier, logger *log.Logger) *Runner {
	return &Runner{cfg: cfg, checker: c, notifier: n, logger: logger}
}

// Run checks every target in order.
//
// A single target never stops the run: an invalid target is reported and
// skipped, and an unreachable one is reported and notified about, after which
// the remaining targets are still checked.
func (r *Runner) Run(ctx context.Context) {
	for i, tgt := range r.cfg.Targets {
		if err := tgt.Validate(); err != nil {
			r.logger.Printf("skipping target %d: %s", i, reasons(err))
			continue
		}

		result, err := r.checker.Check(ctx, tgt.Endpoint, tgt.Threshold)
		if err != nil {
			message := fmt.Sprintf("NG: %s", err)
			r.logger.Print(message)
			r.notify(ctx, tgt, message)
			continue
		}

		message := result.String()
		r.logger.Print(message)
		if result.Expiring {
			r.notify(ctx, tgt, message)
		}
	}
}

// reasons renders a validation error, which joins its causes with newlines,
// on a single log line.
func reasons(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", "; ")
}

// notify delivers message using the Slack settings resolved for tgt.
func (r *Runner) notify(ctx context.Context, tgt config.Target, message string) {
	slack := r.cfg.NotificationFor(tgt)
	err := r.notifier.Notify(ctx, notify.Message{
		WebhookURL: slack.HookURL,
		Channel:    slack.Channel,
		Username:   slack.Username,
		Icon:       slack.Icon,
		Text:       message,
	})
	if err != nil {
		r.logger.Printf("notify %q: %v", tgt.Name, err)
	}
}
