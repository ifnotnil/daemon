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
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSignalReceived(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)), withSTDAPI(s))

	d.signalCh <- os.Interrupt

	d.Wait()
}

func TestSignalsReceivedTriggerOSExit(t *testing.T) {
	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()

	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()
	s.EXPECT().OSExit(2).Run(func(code int) { cnl() }).Once()

	d := Start(
		context.Background(),
		WithMaxSignalCount(2),
		WithLogger(logger(t)),
		withSTDAPI(s),
	)

	// slow shutdown
	d.OnShutDown(func(_ context.Context) {
		sleep(ctx, 1*time.Minute)
	})

	go func() {
		// receive 2 signals, should force immediate shutdown.
		d.signalCh <- os.Interrupt
		d.signalCh <- os.Interrupt
	}()

	d.Wait()
}

func TestFatalErrorReceived(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)), withSTDAPI(s))

	d.FatalErrorsChannel() <- errors.New("error")

	d.Wait()
}

func TestParentContextCancelled(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	ctx, cnl := context.WithCancel(t.Context())
	d := Start(ctx, WithLogger(logger(t)), withSTDAPI(s))

	go cnl()

	d.Wait()
}

func TestShutdownCallbacks(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(
		context.Background(),
		WithLogger(logger(t)),
		withSTDAPI(s),
	)

	// slow shutdown
	m := mock.Mock{}
	defer m.AssertExpectations(t)
	m.Test(t)

	m.On("shutdown_1").Once()
	m.On("shutdown_2").Once()
	m.On("shutdown_3").Once()

	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_1") })
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_2") })
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_3") })

	d.ShutDown()

	d.Wait()
}

func TestShutdownTimeoutExceeded(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(
		context.Background(),
		WithShutdownGraceDuration(10*time.Millisecond),
		WithLogger(logger(t)),
		withSTDAPI(s),
	)

	// slow shutdown
	m := mock.Mock{}
	defer m.AssertExpectations(t)
	m.Test(t)

	m.On("shutdown_1").Once()

	d.OnShutDown(func(ctx context.Context) {
		m.MethodCalled("shutdown_1")
		sleep(ctx, 80*time.Millisecond)
		// ensure that shutdown deadline exceeded.
		assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	})
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_2") })
	d.OnShutDown(func(_ context.Context) { m.MethodCalled("shutdown_3") })

	d.ShutDown()

	d.Wait()
}

func TestCancelCTX(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(context.Background(), WithLogger(logger(t)), withSTDAPI(s))

	m := mock.Mock{}
	defer m.AssertExpectations(t)
	m.Test(t)
	m.On("shutdown_before").Once()
	m.On("shutdown_after").Once()

	d.OnShutDown(func(_ context.Context) {
		m.MethodCalled("shutdown_before")
		assert.NoError(t, d.CTX().Err()) // ensure global context is not canceled
	})
	d.OnShutDown(CancelCTX)
	d.OnShutDown(func(_ context.Context) {
		m.MethodCalled("shutdown_after")
		assert.ErrorIs(t, d.CTX().Err(), context.Canceled)
	})

	d.ShutDown()

	d.Wait()
}

func TestConfigs(t *testing.T) {
	s := newMockstdAPI(t)
	s.EXPECT().SignalNotify(mock.Anything, mock.Anything).Once()
	s.EXPECT().SignalStop(mock.Anything).Once()

	// we specifically want a context that will not get cancelled at the end of the test
	d := Start(t.Context(),
		WithSignalsNotify(os.Interrupt),
		WithMaxSignalCount(42),
		WithFatalErrorsChannelBufferSize(100),
		WithLogger(logger(t)),
		withSTDAPI(s),
	)

	assert.Equal(t, []os.Signal{os.Interrupt}, d.config.signalsNotify)
	assert.Equal(t, 100, cap(d.fatalErrorsCh))
	assert.Equal(t, 42, d.config.maxSignalCount)

	d.ShutDown()
	d.Wait()
}

func TestWithStandardLibrary(t *testing.T) {
	d := Start(t.Context())
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

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// withSTDAPI is used only in testing.
func withSTDAPI(a stdAPI) DaemonConfigOption {
	return func(oc *config) {
		oc.stdAPI = a
	}
}
