// Package broker defines the broker-agnostic message contract used by
// generated code: backends (redis, kafka, mqtt, ...) implement Publisher and
// drive consumers that feed the generated Subscriber.Dispatch.
package broker

import (
	"context"
	"encoding/json"
)

// Incoming is a raw incoming broker message, already bound to a subscription.
//
// Implementations are provided by runtime backends; generated subscriber code
// reads topic, content type, headers and body, and acknowledges on success.
type Incoming interface {
	// Topic returns the address (stream, topic, channel, ...) the message was
	// received from.
	Topic() string
	// Key returns the partition key, if any.
	Key() []byte
	// ContentType returns the message content type; empty means the
	// subscription default (usually application/json).
	ContentType() string
	// Header returns a message header value by name, or "".
	Header(k string) string
	// HeaderNames returns all header names present on the message.
	HeaderNames() []string
	// Body returns the raw payload bytes.
	Body() []byte
	// Ack acknowledges the message. In fire-and-forget backends it is a no-op.
	Ack() error
	// Nack rejects the message; requeue hints the backend to redeliver
	// immediately when supported.
	Nack(requeue bool) error
}

// Outgoing is a raw outgoing broker message.
type Outgoing struct {
	// Topic is the address to publish to (stream, topic, channel, ...).
	Topic string
	// Key is an optional partition key.
	Key string
	// ContentType of the Body.
	ContentType string
	// Headers are message headers, encoded by the publisher.
	Headers map[string]string
	// Body is the raw payload bytes.
	Body []byte
}

// Publisher publishes raw messages to a broker.
type Publisher interface {
	// Publish publishes the message.
	Publish(ctx context.Context, out Outgoing) error
	// Close closes the publisher.
	Close(ctx context.Context) error
}

// Dispatcher dispatches a raw incoming message; implemented by generated
// subscribers (decode → validate → typed handler).
type Dispatcher interface {
	Dispatch(ctx context.Context, raw Incoming) error
}

// HeadersToJSON encodes string headers to JSON object bytes.
func HeadersToJSON(headers map[string]string) []byte {
	if len(headers) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// JSONToHeaders decodes a JSON object of strings.
func JSONToHeaders(data []byte) (map[string]string, error) {
	if len(data) == 0 {
		return map[string]string{}, nil
	}
	headers := map[string]string{}
	if err := json.Unmarshal(data, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

// Handler is a raw message dispatch handler.
type Handler func(ctx context.Context, raw Incoming) error

// Middleware wraps a Handler.
type Middleware func(next Handler) Handler

// Chain builds a single Handler from middlewares; the first middleware is the
// outermost. A nil/empty chain returns the passed handler unchanged.
func Chain(middlewares ...Middleware) func(Handler) Handler {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			mw := middlewares[i]
			if mw == nil {
				continue
			}
			next = mw(next)
		}
		return next
	}
}

// DispatcherFunc adapts a function to Dispatcher.
type DispatcherFunc func(ctx context.Context, raw Incoming) error

// Dispatch implements Dispatcher.
func (f DispatcherFunc) Dispatch(ctx context.Context, raw Incoming) error {
	return f(ctx, raw)
}
