//go:build !darwin

package notification

import (
	_ "embed"
)

//go:embed crush-icon-solo.png
var Icon []byte
