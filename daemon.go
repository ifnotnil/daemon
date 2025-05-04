package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"
)

type daemonCTXKeyType string

const daemonCTXKey = daemonCTXKeyType("daemonCTXKey")

type OnShutDownCallBack func(context.Context)

var CancelCTX OnShutDownCallBack = func(ctx context.Context) {
	a := ctx.Value(daemonCTXKey)
	if d, is := a.(*Daemon); is {
		d.ctxCancel()
	}
}

type config struct {
	signalsNotify                []os.Signal
	maxSignalCount               int
	fatalErrorsChannelBufferSize int
	shutdownTimeout              time.Duration
	logger                       *slog.Logger
	exitFn                       func(code int)
	logSignal                    func(logger *slog.Logger, sig os.Signal)
	logFatalError                func(logger *slog.Logger, err error)
}

// The Daemon struct encapsulates the core functionality required for running an application as a daemon or service, and it ensures a graceful shutdown when stop conditions are met.
// Stop conditions:
//
//	a. A signal (one of daemonConfig.signalsNotify) is received from OS.
//	b. An error is received in fatal errors channel.
//	c. The given parent context (`parentCTX`) in `Start` function is done.
//
// As described in `b` a fatal error channel is returned from the function `FatalErrorsChannel()`, and can be used by the rest of the code when a catastrophic error occurs that needs to trigger an application shutdown.
type Daemon struct {
	config config

	parentCTX context.Context
	ctx       context.Context
	ctxCancel func()

	signalCh      chan os.Signal
	fatalErrorsCh chan error

	onShutDownMutex sync.Mutex
	onShutDown      []func(context.Context)

	shutDownOnce sync.Once

	done chan struct{}
}

// CTX returns the cancelable ctx that will get cancel when the daemon initiates it's shutdown process.
func (o *Daemon) CTX() context.Context { return o.ctx }

func Start(parentCTX context.Context, opts ...DaemonConfigOption) *Daemon {
	cnf := config{
		signalsNotify:                defaultSignals,
		maxSignalCount:               defaultMaxSignalCount,
		fatalErrorsChannelBufferSize: defaultFatalErrorsChannelBufferSize,
		shutdownTimeout:              defaultShutdownTimeout,
		logger:                       slog.New(slog.DiscardHandler),
		exitFn:                       os.Exit,
		logSignal:                    logSignal,
		logFatalError:                logFatalError,
	}

	for _, o := range opts {
		o(&cnf)
	}

	signalCh := make(chan os.Signal, cnf.maxSignalCount)
	signal.Notify(signalCh, cnf.signalsNotify...)

	ctx, ctxCancel := context.WithCancel(parentCTX)
	o := &Daemon{
		config: cnf,

		parentCTX: parentCTX,
		ctx:       ctx,
		ctxCancel: ctxCancel,

		signalCh:      signalCh,
		fatalErrorsCh: make(chan error, cnf.fatalErrorsChannelBufferSize),

		done: make(chan struct{}),
	}

	o.start()

	return o
}

// OnShutDown appends the functions to be called on shutdown after the context gets cancelled.
// The provided functions will be called using a non done context with a timeout configured using `WithShutdownGraceDuration`.
func (o *Daemon) OnShutDown(f ...func(context.Context)) {
	o.onShutDownMutex.Lock()
	defer o.onShutDownMutex.Unlock()
	o.onShutDown = append(o.onShutDown, f...)
}

func (o *Daemon) shutDown() {
	o.config.logger.InfoContext(o.ctx, "starting graceful shutdown")

	pCTX := context.WithValue(o.parentCTX, daemonCTXKey, o)

	// on shutdown, run every shutdown callback with parent ctx and a separate timeout if configured.
	if o.config.shutdownTimeout > 0 {
		dlCTX, dlCancel := context.WithTimeout(pCTX, o.config.shutdownTimeout)
		runWithMutex(dlCTX, &o.onShutDownMutex, o.onShutDown)
		dlCancel()
	} else {
		runWithMutex(pCTX, &o.onShutDownMutex, o.onShutDown)
	}

	// cancel ctx
	o.ctxCancel()

	close(o.done)

	o.config.logger.InfoContext(o.parentCTX, "shutdown completed")
}

// ShutDown will initiate the shutdown process (once) in a separate go routine in order to return immediately.
func (o *Daemon) ShutDown() {
	o.shutDownOnce.Do(func() {
		go o.shutDown()
	})
}

// FatalErrorsChannel returns the fatal error channel that can be used by the application in order to trigger a shutdown.
func (o *Daemon) FatalErrorsChannel() chan<- error {
	return o.fatalErrorsCh
}

// start will spawn a go routine that will run until one of the stop conditions is met.
// After a stop conditions is met the `Daemon` will attempt shutdown "gracefully" by running every function that is registered in `onShutDown` slice, sequentially.
func (o *Daemon) start() {
	go func() {
		sigReceived := 0
		// this loop keeps receiving to ensure that any possible send to signalCh and/or fatalErrorsCh will never block.
		for {
			select {
			// Stop condition (A) signal received.
			case sig := <-o.signalCh:
				sigReceived++
				o.config.logSignal(o.config.logger, sig)
				if o.config.maxSignalCount > 0 && sigReceived >= o.config.maxSignalCount {
					o.config.logger.Error("max number of signal received, terminating immediately")
					o.config.exitFn(defaultImmediateTerminationExitCode)
				}
				o.ShutDown()

			// Stop condition (B) fatal error received.
			case err := <-o.fatalErrorsCh:
				o.config.logFatalError(o.config.logger, err)
				o.ShutDown()

			// stop the loop
			case <-o.done:
				return
			}
		}
	}()

	// Stop condition (C) parent context is done.
	go func() {
		select {
		case <-o.parentCTX.Done():
			err := o.parentCTX.Err()
			s := ""
			if err != nil {
				s = err.Error()
			}
			o.config.logger.Error("parent context got canceled", slog.String("error", s))
			o.ShutDown()
			return

		// stop the loop
		case <-o.done:
			return
		}
	}()
}

func (o *Daemon) Wait() {
	<-o.done
}

type DaemonConfigOption func(*config)

// WithSignalsNotify sets the OS signals that will be used as stop condition to Daemon in order to shutdown gracefully.
func WithSignalsNotify(signals ...os.Signal) DaemonConfigOption {
	return func(oc *config) {
		oc.signalsNotify = signals
	}
}

// WithMaxSignalCount sets the maximum number of signals to receive while waiting for graceful shutdown.
// If the max number of signals exceeds, immediate termination will follow.
func WithMaxSignalCount(size int) DaemonConfigOption {
	return func(oc *config) {
		oc.maxSignalCount = size
	}
}

// WithFatalErrorsChannelBufferSize sets the fatal error channel size in case that is needed to be a buffered one.
func WithFatalErrorsChannelBufferSize(size int) DaemonConfigOption {
	return func(oc *config) {
		oc.fatalErrorsChannelBufferSize = size
	}
}

// WithShutdownGraceDuration sets a timeout to the graceful shutdown process.
// Zero duration means infinite shutdown grace period.
func WithShutdownGraceDuration(d time.Duration) DaemonConfigOption {
	return func(oc *config) {
		oc.shutdownTimeout = d
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) DaemonConfigOption {
	return func(oc *config) {
		oc.logger = l
	}
}

func runWithMutex(ctx context.Context, m *sync.Mutex, fns []func(context.Context)) {
	m.Lock()
	defer m.Unlock()
	for _, f := range fns {
		f(ctx)
		if ctx.Err() != nil {
			return
		}
	}
}
