package daemon

import (
	"context"
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

func logFatalError(ctx context.Context, logger *slog.Logger, err error) {
	logger.ErrorContext(ctx, "fatal error received", slog.String("error", err.Error()))
}

func logSignal(ctx context.Context, logger *slog.Logger, sig os.Signal) {
	signal := slog.String("signal", sig.String())
	signalCode := slog.Attr{}
	if sigInt, ok := sig.(syscall.Signal); ok {
		signalCode = slog.Int("signalCode", int(sigInt))
	}

	logger.WarnContext(ctx, "signal received", signal, signalCode)
}
