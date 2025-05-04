package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSignalReceived(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)))

	d.config.exitFn = exitCalls(t)

	d.signalCh <- os.Interrupt

	d.Wait()
}

func TestSignalReceivedExitFN(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(
		context.Background(),
		WithMaxSignalCount(2),
		WithShutdownGraceDuration(0),
		WithLogger(logger(t)),
	)

	// slow shutdown
	ctx, cnl := context.WithCancel(t.Context())
	defer cnl()
	d.OnShutDown(func(_ context.Context) { sleep(ctx, 1*time.Minute) })

	e := exitCalls(t, 2)
	d.config.exitFn = func(code int) { cnl(); e(code) }

	// receive 2 signals, should force immediate shutdown.
	d.signalCh <- os.Interrupt
	d.signalCh <- os.Interrupt

	d.Wait()
}

func TestFatalErrorReceived(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)))

	d.config.exitFn = exitCalls(t)

	d.FatalErrorsChannel() <- errors.New("error")

	d.Wait()
}

func TestParentContextCancelled(t *testing.T) {
	ctx, cnl := context.WithCancel(t.Context())
	d := Start(ctx, WithLogger(logger(t)))

	d.config.exitFn = exitCalls(t)

	go cnl()

	d.Wait()
}

func TestShutdownTimeout(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(
		context.Background(),
		WithShutdownGraceDuration(10*time.Millisecond),
		WithLogger(logger(t)),
	)

	// slow shutdown
	ctx, cnl := context.WithCancel(t.Context())
	defer cnl()
	m := mock.Mock{}
	defer m.AssertExpectations(t)
	m.Test(t)
	m.On("shutdown_1").Run(func(args mock.Arguments) { sleep(ctx, 60*time.Millisecond) }).Once()
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_1") })
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_2") })
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_3") })

	e := exitCalls(t)
	d.config.exitFn = func(code int) { cnl(); e(code) }

	d.ShutDown()

	d.Wait()
}

func TestContextCancel(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)))

	ctx := d.CTX()

	m := mock.Mock{}
	defer m.AssertExpectations(t)
	m.Test(t)

	m.On("shutdown_before").Run(func(_ mock.Arguments) { require.NoError(t, ctx.Err()) }).Once()
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_before") })

	d.OnShutDown(CancelCTX)

	m.On("shutdown_after").Run(func(_ mock.Arguments) { require.Error(t, ctx.Err()) }).Once()
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_after") })

	d.config.exitFn = exitCalls(t)

	d.ShutDown()

	d.Wait()
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)

	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func exitCalls(t *testing.T, expectedCodes ...int) func(code int) {
	t.Helper()

	m := mock.Mock{}
	m.Test(t)
	for _, c := range expectedCodes {
		m.On("exit", c).Once()
	}
	t.Cleanup(func() { m.AssertExpectations(t) })

	return func(code int) { m.MethodCalled("exit", code) }
}

func TestConfigs(t *testing.T) {
	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(t.Context(), WithSignalsNotify(os.Interrupt), WithFatalErrorsChannelBufferSize(100), WithLogger(logger(t)))

	d.config.exitFn = exitCalls(t)

	assert.Equal(t, []os.Signal{os.Interrupt}, d.config.signalsNotify)
	assert.Equal(t, 100, cap(d.fatalErrorsCh))

	d.ShutDown()
	d.Wait()
}

func logger(t *testing.T) *slog.Logger {
	t.Helper()
	if testing.Verbose() {
		h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false, Level: slog.LevelDebug})
		return slog.New(h).With(slog.String("testcase", t.Name()))
	}

	return slog.New(slog.DiscardHandler)
}
