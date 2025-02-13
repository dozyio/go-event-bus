package eventbus

import (
	"sync"
)

// EventBus represents a basic event bus.
type EventBus struct {
	subscribers map[string][]chan interface{}
	lock        sync.RWMutex
}

// NewEventBus initializes and returns a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan interface{}),
	}
}

// Subscribe registers a new subscriber for a given topic.
// It returns a read-only channel that the subscriber can listen on.
func (bus *EventBus) Subscribe(topic string) <-chan interface{} {
	ch := make(chan interface{})
	bus.lock.Lock()
	bus.subscribers[topic] = append(bus.subscribers[topic], ch)
	bus.lock.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel from the topic.
// It also closes the channel to signal that no more events will be sent.
func (bus *EventBus) Unsubscribe(topic string, subscriber <-chan interface{}) {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	subscribers := bus.subscribers[topic]
	for i, sub := range subscribers {
		if sub == subscriber {
			// Remove the subscriber from the slice.
			bus.subscribers[topic] = append(subscribers[:i], subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// Publish sends the data to all subscribers of the given topic.
// Each subscriber will receive the event concurrently.
func (bus *EventBus) Publish(topic string, data interface{}) {
	bus.lock.RLock()
	defer bus.lock.RUnlock()
	if subscribers, found := bus.subscribers[topic]; found {
		for _, subscriber := range subscribers {
			// Send in a goroutine to avoid blocking other subscribers.
			go func(sub chan interface{}) {
				sub <- data
			}(subscriber)
		}
	}
}
