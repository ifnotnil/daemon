package daemon

import (
	"log/slog"
	"os"
	"syscall"
)

const (
	defaultMaxSignalCount               = 0
	defaultFatalErrorsChannelBufferSize = 10
	defaultShutdownTimeout              = 0
	defaultImmediateTerminationExitCode = 2
)

func logFatalError(logger *slog.Logger, err error) {
	logger.Error("fatal error received", slog.String("error", err.Error()))
}

func logSignal(logger *slog.Logger, sig os.Signal) {
	signal := slog.String("signal", sig.String())
	signalCode := slog.Attr{}
	if sigInt, ok := sig.(syscall.Signal); ok {
		signalCode = slog.Int("signalCode", int(sigInt))
	}

	logger.Warn("signal received", signal, signalCode)
}
