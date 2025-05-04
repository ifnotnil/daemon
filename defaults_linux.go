package daemon

import (
	"os"
	"syscall"
)

var defaultSignals = []os.Signal{os.Interrupt, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM}
