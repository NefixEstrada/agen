// Package redis implements the agen broker runtime for Redis, following the
// agen-specific conventions of DESIGN §6.5 (the official AsyncAPI Redis
// binding is an empty stub):
//
//   - Mode "streams" (default): channel.address is a Redis stream. Publishing
//     XADDs the message with the payload in a "payload" field, the content
//     type in "content_type" and each message header in an "h.<name>" field.
//     Consuming uses XREADGROUP with a consumer group (auto-created with
//     MKSTREAM); successful dispatch XACKs. Handler failures leave entries
//     pending; a reclaimer loop (XAUTOCLAIM) redelivers them up to
//     MaxDeliveries, then (opt-in) dead-letters to "<address>.dlq".
//   - Mode "pubsub": channel.address is a pub/sub channel. Publishing PUBLISHes
//     a JSON envelope; consuming SUBSCRIBEs (or PSUBSCRIBEs for glob
//     patterns). Fire-and-forget: Ack/Nack are no-ops, nothing is persisted.
package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-faster/errors"
	"github.com/redis/go-redis/v9"

	"github.com/NefixEstrada/agen/runtime/broker"
)

// Mode selects the Redis mapping.
type Mode string

// Supported modes.
const (
	// ModeStreams is the durable at-least-once mapping (default).
	ModeStreams Mode = "streams"
	// ModePubSub is the fire-and-forget mapping.
	ModePubSub Mode = "pubsub"
)

// Security configures server authentication.
type Security struct {
	// Username for Redis ACL (spec security scheme userPassword).
	Username string
	// Password for Redis ACL.
	Password string
	// TLS enables TLS (spec security schemes X509, scram256, scram512).
	TLS bool
	// InsecureSkipVerify skips TLS certificate verification.
	InsecureSkipVerify bool
}

func tlsConfig(s Security) *tls.Config {
	if !s.TLS {
		return nil
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.InsecureSkipVerify, //nolint:gosec // opt-in
	}
}

func newClient(addr string, s Security) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:      addr,
		Username:  s.Username,
		Password:  s.Password,
		TLSConfig: tlsConfig(s),
	})
}

// PublisherConfig configures a Redis publisher.
type PublisherConfig struct {
	// Addr of the Redis server, e.g. "redis.example.io:6379".
	Addr string
	// Mode: streams (default) or pubsub.
	Mode Mode
	// Security for the connection.
	Security Security
	// OnError is called on publish failures (optional).
	OnError func(err error)
	// Client overrides Addr/Security when set (tests, shared connections).
	Client redis.UniversalClient
}

// Publisher publishes messages to Redis.
type Publisher struct {
	client redis.UniversalClient
	own    bool
	mode   Mode
	onErr  func(err error)
}

// NewPublisher creates a Redis publisher.
func NewPublisher(cfg PublisherConfig) (*Publisher, error) {
	mode, err := parseMode(cfg.Mode)
	if err != nil {
		return nil, err
	}
	client := cfg.Client
	own := false
	if client == nil {
		if cfg.Addr == "" {
			return nil, errors.New("redis publisher: addr is required")
		}
		client = newClient(cfg.Addr, cfg.Security)
		own = true
	}
	return &Publisher{
		client: client,
		own:    own,
		mode:   mode,
		onErr:  cfg.OnError,
	}, nil
}

func parseMode(m Mode) (Mode, error) {
	switch m {
	case "":
		return ModeStreams, nil
	case ModeStreams, ModePubSub:
		return m, nil
	default:
		return "", errors.Errorf("invalid redis mode %q (streams or pubsub)", m)
	}
}

// Publish implements broker.Publisher.
func (p *Publisher) Publish(ctx context.Context, out broker.Outgoing) error {
	var err error
	switch p.mode {
	case ModePubSub:
		err = p.client.Publish(ctx, out.Topic, encodePubSubEnvelope(out)).Err()
	default:
		err = p.client.XAdd(ctx, &redis.XAddArgs{
			Stream: out.Topic,
			Values: streamValues(out),
		}).Err()
	}
	if err != nil && p.onErr != nil {
		p.onErr(err)
	}
	return err
}

func streamValues(out broker.Outgoing) map[string]any {
	values := map[string]any{
		fieldPayload:     out.Body,
		fieldContentType: out.ContentType,
	}
	if out.Key != "" {
		values[fieldKey] = out.Key
	}
	for k, v := range out.Headers {
		values[fieldHeaderPrefix+k] = v
	}
	return values
}

// Stream field conventions (§6.5).
const (
	fieldPayload      = "payload"
	fieldContentType  = "content_type"
	fieldKey          = "key"
	fieldHeaderPrefix = "h."
)

// pubsubEnvelope is the JSON shape published in pubsub mode.
type pubsubEnvelope struct {
	Payload     []byte            `json:"payload"`
	ContentType string            `json:"content_type"`
	Key         string            `json:"key,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func encodePubSubEnvelope(out broker.Outgoing) string {
	b, err := json.Marshal(pubsubEnvelope{
		Payload:     out.Body,
		ContentType: out.ContentType,
		Key:         out.Key,
		Headers:     out.Headers,
	})
	if err != nil {
		// []byte and string maps always marshal.
		return `{"payload":null}`
	}
	return string(b)
}

// Close implements broker.Publisher.
func (p *Publisher) Close(ctx context.Context) error {
	if p.own {
		return p.client.Close()
	}
	return nil
}

// ConsumerConfig configures a Redis consumer.
type ConsumerConfig struct {
	// Addr of the Redis server.
	Addr string
	// Mode: streams (default) or pubsub.
	Mode Mode
	// Addresses to consume: stream names (streams mode) or channel names
	// and glob patterns (pubsub mode). Required.
	Addresses []string
	// Group is the consumer group name (streams mode). Required in streams
	// mode; corresponds to x-agen-redis.group.
	Group string
	// Consumer name within the group (streams mode).
	Consumer string
	// Security for the connection.
	Security Security
	// Block is the XREADGROUP BLOCK time. Default 5s.
	Block time.Duration
	// Count is the max entries per read. Default 10.
	Count int64
	// ClaimMinIdle is the minimum idle time before reclaiming pending
	// entries (XAUTOCLAIM). Default 30s.
	ClaimMinIdle time.Duration
	// MaxDeliveries dead-letters an entry after this many delivery attempts
	// (streams mode, counted by the local reclaimer). Default 0 = never.
	MaxDeliveries int64
	// DeadLetter enables dead-lettering to "<address>.dlq" (streams mode).
	DeadLetter bool
	// OnError is called for dispatch and loop errors (optional).
	OnError func(err error)
	// Client overrides Addr/Security when set.
	Client redis.UniversalClient
}

// Consumer consumes messages from Redis and dispatches them.
type Consumer struct {
	client redis.UniversalClient
	own    bool
	cfg    ConsumerConfig
	ready  chan struct{}
}

// NewConsumer creates a Redis consumer.
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	mode, err := parseMode(cfg.Mode)
	if err != nil {
		return nil, err
	}
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("redis consumer: no addresses to consume")
	}
	if mode == ModeStreams && cfg.Group == "" {
		return nil, errors.New("redis consumer: streams mode requires a consumer group (ConsumerConfig.Group)")
	}
	cfg.Mode = mode
	if cfg.Block == 0 {
		cfg.Block = 5 * time.Second
	}
	if cfg.Count == 0 {
		cfg.Count = 10
	}
	if cfg.ClaimMinIdle == 0 {
		cfg.ClaimMinIdle = 30 * time.Second
	}
	if cfg.Consumer == "" {
		cfg.Consumer = fmt.Sprintf("consumer-%d", time.Now().UnixNano()%1e6)
	}

	client := cfg.Client
	own := false
	if client == nil {
		if cfg.Addr == "" {
			return nil, errors.New("redis consumer: addr is required")
		}
		client = newClient(cfg.Addr, cfg.Security)
		own = true
	}
	return &Consumer{
		client: client,
		own:    own,
		cfg:    cfg,
		ready:  make(chan struct{}),
	}, nil
}

// Ready is closed once subscriptions and consumer groups are established:
// publishing before readiness may lose messages in pubsub mode.
func (c *Consumer) Ready() <-chan struct{} {
	return c.ready
}

func (c *Consumer) markReady() {
	select {
	case <-c.ready:
	default:
		close(c.ready)
	}
}

// Run consumes until ctx is cancelled, dispatching every message to d.
// A handler error leaves the entry pending (streams mode) for redelivery.
func (c *Consumer) Run(ctx context.Context, d broker.Dispatcher) error {
	switch c.cfg.Mode {
	case ModeStreams:
		return c.runStreams(ctx, d)
	case ModePubSub:
		return c.runPubSub(ctx, d)
	default:
		return errors.Errorf("invalid redis mode %q", c.cfg.Mode)
	}
}

// Close closes the consumer's client when it owns it.
func (c *Consumer) Close(ctx context.Context) error {
	if c.own {
		return c.client.Close()
	}
	return nil
}

func (c *Consumer) onError(err error) {
	if err != nil && c.cfg.OnError != nil {
		c.cfg.OnError(err)
	}
}

func (c *Consumer) runStreams(ctx context.Context, d broker.Dispatcher) error {
	// Ensure the group exists on every stream.
	for _, addr := range c.cfg.Addresses {
		err := c.client.XGroupCreateMkStream(ctx, addr, c.cfg.Group, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return errors.Wrapf(err, "create group %q on %q", c.cfg.Group, addr)
		}
	}

	c.markReady()

	var wg sync.WaitGroup
	if c.cfg.MaxDeliveries > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.runReclaimer(ctx, d)
		}()
	}

	err := c.readLoop(ctx, d)
	wg.Wait()
	return err
}

func (c *Consumer) readLoop(ctx context.Context, d broker.Dispatcher) error {
	streams := append([]string{}, c.cfg.Addresses...)
	streams = append(streams, ">") // undelivered entries only
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.cfg.Group,
			Consumer: c.cfg.Consumer,
			Streams:  streams,
			Count:    c.cfg.Count,
			Block:    c.cfg.Block,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			return errors.Wrap(err, "xreadgroup")
		}
		for _, stream := range result {
			for _, entry := range stream.Messages {
				in := &streamIncoming{
					client: c.client,
					group:  c.cfg.Group,
					stream: stream.Stream,
					entry:  entry,
				}
				if err := d.Dispatch(ctx, in); err != nil {
					// Entry stays pending; the reclaimer redelivers it.
					c.onError(errors.Wrapf(err, "dispatch %s/%s", stream.Stream, entry.ID))
				}
			}
		}
	}
}

func (c *Consumer) runReclaimer(ctx context.Context, d broker.Dispatcher) {
	var (
		cursor   = "0-0"
		attempts = map[string]int64{}
	)
	for {
		if ctx.Err() != nil {
			return
		}
		messages, next, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   c.cfg.Addresses[0],
			Group:    c.cfg.Group,
			Consumer: c.cfg.Consumer,
			MinIdle:  c.cfg.ClaimMinIdle,
			Start:    cursor,
			Count:    c.cfg.Count,
		}).Result()
		if err != nil {
			c.onError(errors.Wrap(err, "xautoclaim"))
			time.Sleep(c.cfg.Block)
			continue
		}
		cursor = next
		for _, entry := range messages {
			attempts[entry.ID]++
			if attempts[entry.ID] >= c.cfg.MaxDeliveries {
				if c.cfg.DeadLetter {
					if err := c.deadLetter(ctx, c.cfg.Addresses[0], entry); err != nil {
						c.onError(errors.Wrap(err, "dead-letter"))
						continue
					}
				}
				_ = c.client.XAck(ctx, c.cfg.Addresses[0], c.cfg.Group, entry.ID)
				delete(attempts, entry.ID)
				continue
			}
			in := &streamIncoming{
				client: c.client,
				group:  c.cfg.Group,
				stream: c.cfg.Addresses[0],
				entry:  entry,
			}
			if err := d.Dispatch(ctx, in); err != nil {
				c.onError(errors.Wrapf(err, "redeliver %s/%s", c.cfg.Addresses[0], entry.ID))
			}
		}
		if len(messages) == 0 {
			time.Sleep(c.cfg.Block)
		}
	}
}

func (c *Consumer) deadLetter(ctx context.Context, stream string, entry redis.XMessage) error {
	values := map[string]any{}
	for k, v := range entry.Values {
		values[k] = v
	}
	values["dlq_source_id"] = entry.ID
	values["dlq_source_stream"] = stream
	return c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream + ".dlq",
		Values: values,
	}).Err()
}

func (c *Consumer) runPubSub(ctx context.Context, d broker.Dispatcher) error {
	// Glob patterns (parameterized channel addresses) use PSUBSCRIBE.
	pubsub := c.client.PSubscribe(ctx, c.cfg.Addresses...)
	defer func() { _ = pubsub.Close() }()

	if _, err := pubsub.Receive(ctx); err != nil {
		return errors.Wrap(err, "psubscribe")
	}
	c.markReady()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return ctx.Err()
			}
			in, err := decodePubSubEnvelope(msg.Channel, msg.Payload)
			if err != nil {
				c.onError(errors.Wrap(err, "decode pubsub message"))
				continue
			}
			if err := d.Dispatch(ctx, in); err != nil {
				c.onError(errors.Wrapf(err, "dispatch %s", msg.Channel))
			}
		}
	}
}

func decodePubSubEnvelope(channel, payload string) (broker.Incoming, error) {
	var env pubsubEnvelope
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		return nil, errors.Wrap(err, "decode envelope")
	}
	return &pubsubIncoming{
		channel:  channel,
		envelope: env,
	}, nil
}

// streamIncoming adapts a Redis stream entry to broker.Incoming.
type streamIncoming struct {
	client redis.UniversalClient
	group  string
	stream string
	entry  redis.XMessage
}

var _ broker.Incoming = (*streamIncoming)(nil)

func (m *streamIncoming) Topic() string       { return m.stream }
func (m *streamIncoming) Key() []byte         { return []byte(str(m.entry.Values[fieldKey])) }
func (m *streamIncoming) ContentType() string { return str(m.entry.Values[fieldContentType]) }

func (m *streamIncoming) Header(k string) string { return str(m.entry.Values[fieldHeaderPrefix+k]) }

func (m *streamIncoming) HeaderNames() []string {
	var names []string
	for k := range m.entry.Values {
		if strings.HasPrefix(k, fieldHeaderPrefix) {
			names = append(names, strings.TrimPrefix(k, fieldHeaderPrefix))
		}
	}
	return names
}

func (m *streamIncoming) Body() []byte {
	switch v := m.entry.Values[fieldPayload].(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return nil
	}
}

// Ack XACKs the entry.
func (m *streamIncoming) Ack() error {
	return m.client.XAck(context.Background(), m.stream, m.group, m.entry.ID).Err()
}

// Nack leaves the entry pending for redelivery.
func (m *streamIncoming) Nack(requeue bool) error { return nil }

// pubsubIncoming adapts a pub/sub message to broker.Incoming.
type pubsubIncoming struct {
	channel  string
	envelope pubsubEnvelope
}

var _ broker.Incoming = (*pubsubIncoming)(nil)

func (m *pubsubIncoming) Topic() string       { return m.channel }
func (m *pubsubIncoming) Key() []byte         { return []byte(m.envelope.Key) }
func (m *pubsubIncoming) ContentType() string { return m.envelope.ContentType }
func (m *pubsubIncoming) Header(k string) string {
	return m.envelope.Headers[k]
}
func (m *pubsubIncoming) HeaderNames() []string {
	names := make([]string, 0, len(m.envelope.Headers))
	for k := range m.envelope.Headers {
		names = append(names, k)
	}
	return names
}
func (m *pubsubIncoming) Body() []byte    { return m.envelope.Payload }
func (m *pubsubIncoming) Ack() error      { return nil }
func (m *pubsubIncoming) Nack(bool) error { return nil }

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}
