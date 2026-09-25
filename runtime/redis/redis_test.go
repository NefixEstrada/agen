package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/runtime/broker"
	redisruntime "github.com/NefixEstrada/agen/runtime/redis"

	streetlights "github.com/NefixEstrada/agen/internal/integration/streetlights"
)

func setupRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func dispatcher(sub *streetlights.Subscriber) broker.Dispatcher {
	return broker.DispatcherFunc(sub.Dispatch)
}

func TestStreamsPublishConsume(t *testing.T) {
	client := setupRedis(t)

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	require.NoError(t, pub.Publish(context.Background(), broker.Outgoing{
		Topic:       "streetlights.2.light",
		ContentType: "application/json",
		Headers:     map[string]string{"requestId": "abc"},
		Body:        []byte(`{"lumens":42}`),
	}))

	got := make(chan *streetlights.LightMeasured, 1)
	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(
		lightHandler(func(ctx context.Context, msg *streetlights.LightMeasured) error {
			got <- msg
			return nil
		}),
	))

	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Addresses: []string{"streetlights.2.light"},
		Group:     "app",
		Block:     100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, dispatcher(sub)) }()

	select {
	case msg := <-got:
		require.Equal(t, "abc", msg.Headers.RequestId.Value)
		require.Equal(t, int(42), msg.Payload.Lumens.Value)
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
	}

	// Successful dispatch XACKs.
	require.Eventually(t, func() bool {
		pending, err := client.XPending(ctx, "streetlights.2.light", "app").Result()
		return err == nil && pending.Count == 0
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	<-done
}

func TestStreamsFailedDispatchStaysPending(t *testing.T) {
	client := setupRedis(t)

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	require.NoError(t, pub.Publish(context.Background(), broker.Outgoing{
		Topic:       "streetlights.2.light",
		ContentType: "application/json",
		Body:        []byte(`{"lumens":"bogus"}`), // fails decode
	}))

	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(
		lightHandler(func(ctx context.Context, msg *streetlights.LightMeasured) error {
			t.Error("handler must not be called for invalid message")
			return nil
		}),
	))

	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Addresses: []string{"streetlights.2.light"},
		Group:     "app",
		Block:     100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, dispatcher(sub)) }()

	// Invalid entry stays pending (not XACKed).
	require.Eventually(t, func() bool {
		pending, err := client.XPending(ctx, "streetlights.2.light", "app").Result()
		return err == nil && pending.Count == 1
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	<-done
}

func TestSendOperationEndToEnd(t *testing.T) {
	client := setupRedis(t)

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	c, err := streetlights.NewClient("", streetlights.WithPublisher(pub))
	require.NoError(t, err)
	require.NoError(t, c.SendLightCommand(context.Background(), &streetlights.LightCommand{
		Payload: streetlights.LightCommandPayload{
			Command: streetlights.NewOptLightCommandPayloadCommand(streetlights.LightCommandPayloadCommandOn),
		},
	}, streetlights.WithKey("k1"), streetlights.WithHeader("traceId", "t1")))

	entries, err := client.XRange(context.Background(), streetlights.LightCommandAddress, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	values := entries[0].Values
	require.Equal(t, `{"command":"on"}`, values["payload"])
	require.Equal(t, "application/json", values["content_type"])
	require.Equal(t, "k1", values["key"])
	require.Equal(t, "t1", values["h.traceId"])
}

func TestPubSubRoundTrip(t *testing.T) {
	client := setupRedis(t)

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{
		Client: client,
		Mode:   redisruntime.ModePubSub,
	})
	require.NoError(t, err)

	got := make(chan *streetlights.LightMeasured, 1)
	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(
		lightHandler(func(ctx context.Context, msg *streetlights.LightMeasured) error {
			got <- msg
			return nil
		}),
	))

	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Mode:      redisruntime.ModePubSub,
		Addresses: []string{"streetlights.*"},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, dispatcher(sub)) }()
	time.Sleep(150 * time.Millisecond) // let the subscription settle

	require.NoError(t, pub.Publish(context.Background(), broker.Outgoing{
		Topic:       "streetlights.9.light",
		ContentType: "application/json",
		Headers:     map[string]string{"requestId": "r1"},
		Body:        []byte(`{"lumens":7}`),
	}))

	select {
	case msg := <-got:
		require.Equal(t, "r1", msg.Headers.RequestId.Value)
		require.Equal(t, int(7), msg.Payload.Lumens.Value)
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
	}

	cancel()
	<-done
}

type lightHandlerFn func(ctx context.Context, msg *streetlights.LightMeasured) error

func (f lightHandlerFn) ReceiveLightMeasurement(ctx context.Context, msg *streetlights.LightMeasured) error {
	return f(ctx, msg)
}

func lightHandler(f lightHandlerFn) streetlights.ReceiveLightMeasurementHandler { return f }
