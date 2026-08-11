//go:build !solaris && !illumos

package notification

import "github.com/gen2brain/beeep"

// NativeSupported reports whether native OS notifications are available on
// this platform. It is false on illumos/solaris, where beeep's dbus
// dependency does not build. Callers use it to hide the native option and
// pick a terminal-based backend instead.
const NativeSupported = true

// defaultNotifyFunc delivers notifications through beeep on platforms where
// its dependencies build.
var defaultNotifyFunc = beeep.Notify

func init() {
	beeep.AppName = "Crush"
}
