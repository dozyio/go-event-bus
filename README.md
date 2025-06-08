# go-event-bus

A lightweight, thread-safe publish/subscribe event bus for Go, featuring:

- **Graceful shutdown**: prevents new publishes, waits for in-flight deliveries, and cleanly closes all subscriber channels.
- **Panic isolation**: recovers from panics in subscriber handlers so one bad subscriber can’t crash your application.
- **Synchronous and Asynchronous publishes**: `PublishSync` blocks until all subscribers have received the event (or panicked).
- **Introspection**: list active topics and subscriber counts.

---

## Installation

```bash
go get github.com/dozyio/go-event-bus
````

```go
import "github.com/dozyio/go-event-bus"
```

---

## Quick Start

```go
package main

import (
    "fmt"
    "time"

    eventbus "github.com/dozyio/go-event-bus"
)

func main() {
    bus := eventbus.New()

    // Subscriber 1
    ch1 := bus.Subscribe("topic:hello")
    go func() {
        for msg := range ch1 {
            fmt.Printf("Subscriber1 got: %v\n", msg)
        }
        fmt.Println("Subscriber1 channel closed")
    }()

    // Subscriber 2
    ch2 := bus.Subscribe("topic:hello")
    go func() {
        for msg := range ch2 {
            fmt.Printf("Subscriber2 got: %v\n", msg)
        }
        fmt.Println("Subscriber2 channel closed")
    }()

    // Publish asynchronously (non-blocking)
    bus.Publish("topic:hello", "👋 Hello, EventBus!")

    // Publish synchronously (blocks until both subscribers read)
    if err := bus.PublishSync("topic:hello", "🔒 Synchronous message"); err != nil {
        fmt.Println("PublishSync error:", err)
    }

    // Inspect topics & counts
    fmt.Println("Active topics:", bus.Topics())
    fmt.Printf("Subscribers for topic:hello = %d\n", bus.SubscriberCount("topic:hello"))

    // Graceful shutdown
    bus.Close()

    // Give subscribers time to print
    time.Sleep(100 * time.Millisecond)
}
```

---

## API Reference

### `func New() *EventBus`

Creates and returns a new `EventBus`.

### `func (bus *EventBus) Subscribe(topic string) <-chan interface{}`

Registers a new subscriber for `topic`. Returns a read-only channel; events published to `topic` will be sent here. The channel is closed when:

* The subscriber is explicitly unsubscribed via `Unsubscribe`.
* The bus is closed via `Close`.

### `func (bus *EventBus) Unsubscribe(topic string, subscriber <-chan interface{})`

Removes the given subscriber channel from `topic`, closes it, and will no longer receive events.

### `func (bus *EventBus) Publish(topic string, data interface{})`

Asynchronously publishes `data` to all subscribers of `topic`. Each delivery runs in its own goroutine, with panics recovered and logged. Returns immediately.

### `func (bus *EventBus) PublishSync(topic string, data interface{}) error`

Synchronously publishes `data` to all subscribers of `topic` in sequence:

* Blocks until each subscriber’s channel has been sent to.
* Recovers and records any panics from subscribers.
* Returns `eventbus.ErrBusClosed` if the bus is already closed.
* If any subscriber panicked, returns an aggregated error.

### `func (bus *EventBus) Close()`

Gracefully shuts down the bus:

1. Marks the bus as closed (future publishes are no-ops).
2. Unblocks any in-flight `Publish` calls.
3. Waits for all delivery goroutines to finish.
4. Closes all subscriber channels and clears the topic map.

Safe to call multiple times.

### `func (bus *EventBus) Topics() []string`

Returns a slice of all currently registered topic names. Note: topic entries remain until `Close` is called, even if they have zero subscribers.

### `func (bus *EventBus) SubscriberCount(topic string) int`

Returns the number of active subscribers for `topic`.

---

## Testing

The repository includes a comprehensive test suite covering:

* Basic publish/subscribe behavior
* Unsubscribe and channel-closing
* `PublishSync` happy and error paths
* Graceful shutdown semantics
* Panic isolation

Run all tests (with race detection):

```bash
go test . -v -race
```

---

## Contributing

1. Fork the repo
2. Create a feature branch
3. Write tests for new behavior
4. Submit a pull request

---

## License

MIT
