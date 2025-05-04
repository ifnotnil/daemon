//go:build !linux

package daemon

import (
	"os"
)

var defaultSignals = []os.Signal{os.Interrupt}
