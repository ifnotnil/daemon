// Package daemon encapsulates the core functionality required for running an application as a daemon or service,
// and it ensures a graceful shutdown when stop conditions are met.
//
// Stop conditions:
//  1. A signal (one of daemonConfig.signalsNotify) is received from OS.
//  2. An error is received in fatal errors channel.
//  3. the given parent context (parentCTX) in Start function is done.
//
// The shutdown can be initiated manually at any point by calling ShutDown() daemon's receiver function.
//
// Example usage:
//
//	func main() {
//		d := daemon.Start(
//			context.Background(),
//			daemon.WithSignalsNotify(os.Interrupt, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM),
//			daemon.WithShutdownGraceDuration(5*time.Second),
//		)
//
//		ctx := d.CTX() // This ctx should be provided to the rest of the code
//
//		// example modules
//		db := InitRepo(ctx)
//		serviceA := InitServiceA(ctx, db) // starts its own go routines / jobs
//		httpServer := NewHTTPModule(ctx, serviceA) // starts its own go routine
//		consumers := InitQueueConsumer(ctx) // starts its own go routine
//
//		d.Defer(
//			httpServer.ShutDown,
//			consumers.Stop,
//			serviceA.Stop,
//			db.Stop,
//		)
//
//		d.Wait() // this will block until the graceful shutdown is initiated and done.
//	}
//
// Context:
// The context provided by the daemon struct .CTX() should be passed downstream to the rest of the code.
// It will get cancelled by default after the shutdown callbacks are done or if it configured as a shutdown callback
// by passing daemon.CancelCTX in the Defer() function.
//
// Shutdown callbacks:
// Using the daemon function Defer(f ...func(context.Context)) you can register callback functions that will be called
// (in LIFO order) once the graceful shutdown is initiated. The context that is given to each shutdown callback is not the same with .CTX().
// It will be the parentCTX with a separate timeout (shutdown grace period) depending on the configuration.
//
// Fatal errors channel:
// Daemon provides an error channel FatalErrorsChannel() chan<- error that can be used downstream to push errors
// that are considered catastrophic into it. Once an error received in this channel the daemon struct will initiate
// the graceful shutdown process.
package daemon
