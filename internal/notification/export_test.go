package notification

import "github.com/gen2brain/beeep"

// SetNotifyFunc allows replacing the notification function for testing.
func SetNotifyFunc(fn func(string, string, any) error) {
	notifyFunc = fn
}

// ResetNotifyFunc resets the notification function to the default.
func ResetNotifyFunc() {
	notifyFunc = beeep.Notify
}
