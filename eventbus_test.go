// eventbus_test.go
package eventbus_test

import (
	"slices"
	"testing"
	"time"

	eventbus "github.com/dozyio/go-event-bus"
)

// receive is a helper that reads from ch or fails the test after timeout.
func receive(t *testing.T, ch <-chan any, timeout time.Duration) (any, bool) {
	t.Helper()
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for message")
		return nil, false
	}
}

func TestPublishSubscribe(t *testing.T) {
	bus := eventbus.NewEventBus()

	ch := bus.Subscribe("foo")
	bus.Publish("foo", "hello")

	v, ok := receive(t, ch, 200*time.Millisecond)
	if !ok {
		t.Fatal("expected channel to be open")
	}
	if vStr, _ := v.(string); vStr != "hello" {
		t.Fatalf("got %v; want %q", v, "hello")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := eventbus.NewEventBus()

	ch1 := bus.Subscribe("topic")
	ch2 := bus.Subscribe("topic")

	bus.Unsubscribe("topic", ch1)

	// ch1 should be closed
	if _, ok := <-ch1; ok {
		t.Fatal("expected ch1 to be closed after unsubscribe")
	}

	// publishing still delivers to ch2
	bus.Publish("topic", 42)
	v, ok := receive(t, ch2, 200*time.Millisecond)
	if !ok {
		t.Fatal("expected ch2 to be open")
	}
	if vInt, _ := v.(int); vInt != 42 {
		t.Fatalf("got %v; want 42", v)
	}
}

func TestSubscriberCountAndTopics(t *testing.T) {
	bus := eventbus.NewEventBus()

	// initially empty
	if c := bus.SubscriberCount("x"); c != 0 {
		t.Fatalf("empty bus: got count %d; want 0", c)
	}
	if ts := bus.Topics(); len(ts) != 0 {
		t.Fatalf("empty bus: got topics %v; want []", ts)
	}

	// subscribe two on "A" and one on "B"
	chA1 := bus.Subscribe("A")
	chA2 := bus.Subscribe("A")
	_ = bus.Subscribe("B")

	if c := bus.SubscriberCount("A"); c != 2 {
		t.Fatalf("A count: got %d; want 2", c)
	}
	if c := bus.SubscriberCount("B"); c != 1 {
		t.Fatalf("B count: got %d; want 1", c)
	}

	ts := bus.Topics()
	if len(ts) != 2 || !slices.Contains(ts, "A") || !slices.Contains(ts, "B") {
		t.Fatalf("topics %v; want [A B] in any order", ts)
	}

	// unsubscribe both from "A"
	bus.Unsubscribe("A", chA1)
	bus.Unsubscribe("A", chA2)
	if c := bus.SubscriberCount("A"); c != 0 {
		t.Fatalf("after unsubscribing both, A count: got %d; want 0", c)
	}

	// map keys persist until Close, so Topics still shows "A" and "B"
	ts2 := bus.Topics()
	if len(ts2) != 2 || !slices.Contains(ts2, "A") || !slices.Contains(ts2, "B") {
		t.Fatalf("after removing all A subs, topics %v; want still [A B]", ts2)
	}
}

func TestCloseBehavior(t *testing.T) {
	bus := eventbus.NewEventBus()

	ch1 := bus.Subscribe("t1")
	ch2 := bus.Subscribe("t2")

	// kick off one in-flight publish (will block until quitCh closed)
	bus.Publish("t1", "x")

	done := make(chan struct{})
	go func() {
		bus.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close did not return promptly")
	}

	// all channels closed
	if _, ok := <-ch1; ok {
		t.Error("ch1 should be closed after Close()")
	}
	if _, ok := <-ch2; ok {
		t.Error("ch2 should be closed after Close()")
	}

	// introspection now empty
	if ts := bus.Topics(); len(ts) != 0 {
		t.Fatalf("after Close, got topics %v; want []", ts)
	}
	if c := bus.SubscriberCount("t1"); c != 0 {
		t.Fatalf("after Close, count for t1 = %d; want 0", c)
	}
}

func TestSubscribeAfterClose(t *testing.T) {
	bus := eventbus.NewEventBus()
	bus.Close()

	ch := bus.Subscribe("any")
	if _, ok := <-ch; ok {
		t.Fatal("subscribe after Close should return closed channel")
	}
}

func TestPublishAfterClose(t *testing.T) {
	bus := eventbus.NewEventBus()
	bus.Close()
	// should not panic or block
	bus.Publish("foo", "bar")
}

func TestPublishSync(t *testing.T) {
	bus := eventbus.NewEventBus()

	ch1 := bus.Subscribe("sync")
	ch2 := bus.Subscribe("sync")

	errCh := make(chan error, 1)
	go func() {
		errCh <- bus.PublishSync("sync", "syncmsg")
	}()

	v1, ok1 := receive(t, ch1, 200*time.Millisecond)
	if !ok1 || v1.(string) != "syncmsg" {
		t.Fatalf("subscriber1: got %v (open=%v), want %q", v1, ok1, "syncmsg")
	}
	v2, ok2 := receive(t, ch2, 200*time.Millisecond)
	if !ok2 || v2.(string) != "syncmsg" {
		t.Fatalf("subscriber2: got %v (open=%v), want %q", v2, ok2, "syncmsg")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("PublishSync returned error: %v", err)
	}
}

func TestPublishSyncAfterClose(t *testing.T) {
	bus := eventbus.NewEventBus()
	bus.Close()

	err := bus.PublishSync("any", 123)
	if err != eventbus.ErrBusClosed {
		t.Fatalf("after Close(), PublishSync returned %v; want ErrBusClosed", err)
	}
}
