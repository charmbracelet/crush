package notification

// Sink is a function that accepts notification requests.
// This allows agents to publish notifications without knowing about the UI.
type Sink func(Notification)

// NewChannelSink creates a Sink that sends notifications to a channel.
// The channel should be buffered to avoid blocking the caller.
func NewChannelSink(ch chan<- Notification) Sink {
	return func(n Notification) {
		select {
		case ch <- n:
		default:
			// Channel full, drop the notification to avoid blocking.
		}
	}
}
