package orders_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/runtime/agenerrors"
	"github.com/NefixEstrada/agen/runtime/broker"
	redisruntime "github.com/NefixEstrada/agen/runtime/redis"

	orders "github.com/NefixEstrada/agen/internal/integration/orders"
)

// The orders spec exercises: multi-message channels (sum dispatch),
// parameterized addresses with enum validation, fakes and unimplemented.

func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	return miniredis.RunT(t)
}

func brokerDispatch(sub *orders.Subscriber) broker.Dispatcher {
	return broker.DispatcherFunc(sub.Dispatch)
}

func outgoing(topic, body string) broker.Outgoing {
	return broker.Outgoing{Topic: topic, ContentType: "application/json", Body: []byte(body)}
}

func TestMultiMessageDispatch(t *testing.T) {
	mr := setupMiniredis(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	// Decoders are lenient (unknown fields are skipped), so a payload with
	// just orderId is ambiguous and dispatches to the first variant tried
	// (messages are tried in sorted spec order: orderCancelled first).
	require.NoError(t, pub.Publish(context.Background(), outgoing("orders.events.acme", `{"orderId":"o1"}`)))

	got := make(chan orders.OrderEventsMessage, 1)
	sub := orders.NewSubscriber(orders.NewSubscriberHandlers(
		eventHandler(func(ctx context.Context, msg orders.OrderEventsMessage) error {
			got <- msg
			return nil
		}),
	))
	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Addresses: []string{"orders.events.acme"},
		Group:     "app",
		Block:     100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, brokerDispatch(sub)) }()

	var msg orders.OrderEventsMessage
	select {
	case msg = <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
	}
	cancel()
	<-done

	cancelled, ok := msg.(*orders.OrderCancelled)
	require.True(t, ok, "ambiguous shape must dispatch to the first variant, got %T", msg)
	require.Equal(t, "o1", cancelled.Payload.OrderId)
}

func TestMultiMessageDispatchCancelled(t *testing.T) {
	mr := setupMiniredis(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	// reason is unique to OrderCancelled, so the created variant fails decode.
	require.NoError(t, pub.Publish(context.Background(),
		outgoing("orders.events.globex", `{"orderId":"o2","reason":"payment"}`)))

	got := make(chan orders.OrderEventsMessage, 1)
	sub := orders.NewSubscriber(orders.NewSubscriberHandlers(
		eventHandler(func(ctx context.Context, msg orders.OrderEventsMessage) error {
			got <- msg
			return nil
		}),
	))
	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Addresses: []string{"orders.events.globex"},
		Group:     "app",
		Block:     100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, brokerDispatch(sub)) }()

	var msg orders.OrderEventsMessage
	select {
	case msg = <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
	}
	cancel()
	<-done

	cancelled, ok := msg.(*orders.OrderCancelled)
	require.True(t, ok, "dispatched %T", msg)
	require.Equal(t, "payment", cancelled.Payload.Reason.Value)
}

func TestParameterizedPublish(t *testing.T) {
	mr := setupMiniredis(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	pub, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	c := orders.NewClient(pub)

	require.NoError(t, c.SendOrderEvents(context.Background(), "acme", &orders.OrderCreated{
		Payload: orders.OrderCreatedPayload{
			OrderId: "o3",
		},
	}))

	entries, err := client.XRange(context.Background(), "orders.events.acme", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Invalid tenant must be rejected before reaching Redis.
	err = c.SendOrderEvents(context.Background(), "bogus", &orders.OrderCreated{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid tenantId")
}

func TestFakesAndUnimplemented(t *testing.T) {
	fake := &orders.FakePublisher{}
	c := orders.NewClient(fake)
	require.NoError(t, c.SendOrderEvents(context.Background(), "acme", &orders.OrderCreated{}))
	require.Len(t, fake.Messages, 1)
	require.Equal(t, "orders.events.acme", fake.Messages[0].Topic)

	fakeHandler := &orders.FakeReceiveOrderEventsHandler{}
	err := fakeHandler.ReceiveOrderEvents(context.Background(), &orders.OrderCreated{})
	require.NoError(t, err)
	require.Len(t, fakeHandler.Messages, 1)

	unimplemented := orders.UnimplementedHandler{}
	err = unimplemented.ReceiveOrderEvents(context.Background(), &orders.OrderCreated{})
	require.ErrorIs(t, err, agenerrors.ErrNotImplemented)
}

type eventHandlerFn func(ctx context.Context, msg orders.OrderEventsMessage) error

func (f eventHandlerFn) ReceiveOrderEvents(ctx context.Context, msg orders.OrderEventsMessage) error {
	return f(ctx, msg)
}

func eventHandler(f eventHandlerFn) orders.ReceiveOrderEventsHandler { return f }
