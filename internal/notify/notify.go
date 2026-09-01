// Package notify delivers certcheck messages to notification services.
//
// Callers depend on the [Notifier] interface rather than on a concrete
// service, so a new destination can be added without touching the code that
// decides what to send.
package notify

import "context"

// Message is a single notification to deliver.
//
// Every field is already resolved by the caller: the package does not know
// about configuration files or per-target overrides.
type Message struct {
	// WebhookURL is the endpoint the message is delivered to.
	WebhookURL string
	// Channel is the channel to post in. Empty means the webhook default.
	Channel string
	// Username is the display name of the posting user. Empty means the
	// webhook default.
	Username string
	// Icon is the emoji shown next to the message. Empty means the webhook
	// default.
	Icon string
	// Text is the message body.
	Text string
}

// Notifier delivers a [Message].
type Notifier interface {
	Notify(ctx context.Context, msg Message) error
}
