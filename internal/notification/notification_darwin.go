//go:build darwin

package notification

// notificationIcon is empty on darwin because icon support is broken.
var notificationIcon interface{} = ""
