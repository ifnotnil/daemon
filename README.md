# daemon
[![CI Status](https://github.com/ifnotnil/daemon/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ifnotnil/daemon/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/ifnotnil/daemon/graph/badge.svg)](https://codecov.io/gh/ifnotnil/daemon)
[![Go Report Card](https://goreportcard.com/badge/github.com/ifnotnil/daemon)](https://goreportcard.com/report/github.com/ifnotnil/daemon)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/ifnotnil/daemon)](https://pkg.go.dev/github.com/ifnotnil/daemon)

Install:
```shell
go get -u github.com/ifnotnil/daemon
```

The daemon package encapsulates the core functionality required for running an application as a daemon or service, and it ensures a graceful shutdown when stop conditions are met.
Stop conditions:

  1. A signal (one of daemonConfig.signalsNotify) is received from OS.
  2. An error is received in fatal errors channel.
  3. the given parent context (`parentCTX`) in `Start` function is done.

The shutdown can be initiated manually at any point by calling `ShutDown()` daemon's receiver function.

Example:

```golang
func main() {
	d := daemon.Start(
		context.Background(),
		daemon.WithSignalsNotify(os.Interrupt, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM),
		daemon.WithShutdownGraceDuration(5*time.Second),
	)

	ctx := d.CTX() // This ctx should be provided to the rest of the code

	// example modules
	db := InitRepo(ctx)
	serviceA := InitServiceA(ctx, db) // starts its own go routines / jobs
	httpServer := NewHTTPModule(ctx, serviceA) // starts its own go routine
	consumers := InitQueueConsumer(ctx) // starts its own go routine

	d.Defer(
		db.Stop,
		serviceA.Stop,
		consumers.Stop,
		httpServer.ShutDown,
	)

	d.Wait() // this will block ti the graceful shutdown is initiated and done.
}
```

### Context
The context provided by the daemon struct `.CTX()` should be passed downstream to the rest of the code.

It will get cancelled by default after the shutdown callbacks are done **or** if it configured as a shutdown callback by passing `daemon.CancelCTX` in the `Defer()` function. For example:
```golang
	d.Defer(
		db.Stop,
		daemon.CancelCTX  // the ctx will be cancelled before the db disconnects.
		serviceA.Stop,
		consumers.Stop,
		httpServer.ShutDown,
	)
```

### Defer(...)
Using the daemon function `Defer(f ...func(context.Context))` you can register callback functions that will be called once the graceful shutdown is initiated.

Callbacks registered in `Defer` will be called in the reverse order they are registered (like `defer` keyword).

e.g.
```golang
d.Defer(
	fifth,
	fourth,
)
// ...
d.Defer(
	third,
	second,
	fist,
)
```

The context that is given to each shutdown callback is not the same with `.CTX()`. It will be the `parentCTX` with a separate timeout (shutdown grace period) depending on the configuration.

Using the `WithShutdownGraceDuration` option you can set the grace period of shutdown, after which the ctx given to each shutdown callback will be cancelled. By setting to `0`, infinite grace period is set.

### Fatal errors channel
Daemon provides an error channel `FatalErrorsChannel() chan<- error` that can be used downstream to push errors that are considered catastrophic into it. Once an error received in this channel the daemon struct will initiate the graceful shutdown process.
