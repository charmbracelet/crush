//go:build !darwin

package notification

import (
	_ "embed"
)

//go:embed crush-icon-solo.png
var icon []byte

// notificationIcon contains the embedded PNG icon data for desktop notifications.
var notificationIcon interface{} = icon
