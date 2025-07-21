package daemon

import (
	"os"
	"os/signal"
)

type std struct{}

func (std) SignalStop(c chan<- os.Signal) {
	signal.Stop(c)
}

func (std) SignalNotify(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}

func (std) OSExit(code int) {
	os.Exit(code)
}
