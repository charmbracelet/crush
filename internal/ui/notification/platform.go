package notification

// NativeSupported is false because Ultra uses terminal-native OSC or bell
// notifications instead of platform-specific desktop integrations.
const NativeSupported = false

var defaultNotifyFunc = func(string, string, any) error { return nil }

// Icon is intentionally empty; terminal notifications do not embed product
// artwork.
var Icon []byte
