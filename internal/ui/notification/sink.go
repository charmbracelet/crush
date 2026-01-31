package notification

// Sink is a function that accepts notification requests.
// This allows agents to publish notifications without knowing about the UI.
type Sink func(Notification)

// NewChannelSink creates a Sink that sends notifications to a channel. The
// channel should have a buffer of 1.
//
// Any pending notification is discarded before sending the new one. This
// ensures the consumer always sees the most recent notification rather
// than a potential barrage when only one is needed.
func NewChannelSink(ch chan Notification) Sink {
	return func(n Notification) {
		// Drain any existing notification.
		select {
		case <-ch:
		default:
		}
		// Send the new notification. The channel should be empty, but it uses a
		// non-blocking send for safety in case of a race with the consumer.
		select {
		case ch <- n:
		default:
		}
	}
}
