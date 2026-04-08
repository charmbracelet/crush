//go:build darwin

package notification

// Icon is currently empty on darwin because platform icon support is broken. Do
// use the icon for OSC notifications, just not native.
var Icon any = ""
