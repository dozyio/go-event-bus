// eventbus.go
package eventbus

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
)

// EventBus represents a basic event bus with graceful shutdown,
// panic-isolation, and introspection capabilities.
type EventBus struct {
	subscribers map[string][]chan any
	lock        sync.RWMutex
	wg          sync.WaitGroup
	closed      bool
	quitCh      chan struct{}
}

// ErrBusClosed is returned by PublishSync if the bus has been closed.
var ErrBusClosed = errors.New("eventbus: bus closed")

// NewEventBus initializes and returns a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan any),
		quitCh:      make(chan struct{}),
	}
}

// Subscribe registers a new subscriber for a given topic.
// It returns a read-only channel that the subscriber can listen on.
func (bus *EventBus) Subscribe(topic string) <-chan any {
	ch := make(chan any)
	bus.lock.Lock()
	defer bus.lock.Unlock()

	if bus.closed {
		close(ch)
		return ch
	}
	bus.subscribers[topic] = append(bus.subscribers[topic], ch)
	return ch
}

// Unsubscribe removes a subscriber channel from the topic.
// It also closes the channel to signal that no more events will be sent.
func (bus *EventBus) Unsubscribe(topic string, subscriber <-chan any) {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	subs := bus.subscribers[topic]
	for i, sub := range subs {
		if sub == subscriber {
			bus.subscribers[topic] = slices.Delete(subs, i, i+1)
			close(sub)
			break
		}
	}
}

// Publish sends the data to all subscribers of the given topic.
// Each subscriber receives the event in its own goroutine, with panic isolation.
// If the bus has been closed, Publish is a no-op.
// During shutdown, any blocked sends are unblocked via quitCh rather than a channel close.
func (bus *EventBus) Publish(topic string, data any) {
	bus.lock.RLock()
	if bus.closed {
		bus.lock.RUnlock()
		return
	}
	subs := slices.Clone(bus.subscribers[topic])
	bus.lock.RUnlock()

	for _, sub := range subs {
		bus.wg.Add(1)
		go func(ch chan any) {
			defer bus.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("eventbus: recovered from subscriber panic: %v", r)
				}
			}()
			select {
			case ch <- data:
			case <-bus.quitCh:
			}
		}(sub)
	}
}

// PublishSync sends the data to all subscribers of the given topic
// synchronously, one after the other, waiting for each send to complete.
// It recovers from panics (e.g. if someone manually closed a subscriber-ch).
// If the bus is closed, it returns ErrBusClosed immediately.
// If any subscriber panicked, it returns an aggregated error.
func (bus *EventBus) PublishSync(topic string, data any) error {
	bus.lock.RLock()
	if bus.closed {
		bus.lock.RUnlock()
		return ErrBusClosed
	}
	subs := slices.Clone(bus.subscribers[topic])
	bus.lock.RUnlock()

	var errs []error
	for i, sub := range subs {
		func(ch chan any) {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, fmt.Errorf("subscriber %d panic: %v", i, r))
				}
			}()
			ch <- data
		}(sub)
	}
	if len(errs) > 0 {
		return fmt.Errorf("PublishSync: %d error(s); first: %v", len(errs), errs[0])
	}
	return nil
}

// Close gracefully shuts down the bus in three phases:
// 1. Mark as closed and close quitCh (unblocking any in-flight Publish calls).
// 2. Wait for all Publish goroutines to exit.
// 3. Close all subscriber channels and clear the subscriber map.
func (bus *EventBus) Close() {
	bus.lock.Lock()
	if bus.closed {
		bus.lock.Unlock()
		return
	}
	bus.closed = true
	close(bus.quitCh)
	bus.lock.Unlock()

	// wait for all in-flight Publish goroutines to finish
	bus.wg.Wait()

	// now safe to close subscriber channels (no sends in flight)
	bus.lock.Lock()
	for topic, subs := range bus.subscribers {
		for _, sub := range subs {
			close(sub)
		}
		delete(bus.subscribers, topic)
	}
	bus.lock.Unlock()
}

// Topics returns a slice of all currently active topic names.
func (bus *EventBus) Topics() []string {
	bus.lock.RLock()
	defer bus.lock.RUnlock()

	topics := make([]string, 0, len(bus.subscribers))
	for t := range bus.subscribers {
		topics = append(topics, t)
	}
	return topics
}

// SubscriberCount returns the number of subscribers for a given topic.
func (bus *EventBus) SubscriberCount(topic string) int {
	bus.lock.RLock()
	defer bus.lock.RUnlock()
	return len(bus.subscribers[topic])
}
