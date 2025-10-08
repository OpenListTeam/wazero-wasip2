package v0_2

import (
	"context"
	"time"

	"github.com/foxxorcat/wazero-wasip2/manager/io"
)

// Monotonic clock zero value, initialized at package load time.
var programStart = time.Now()

type monotonicClockImpl struct {
	pm *io.PollManager
}

func newMonotonicClockImpl(pm *io.PollManager) *monotonicClockImpl {
	return &monotonicClockImpl{pm: pm}
}

// Now returns the current time from the monotonic clock in nanoseconds.
func (i *monotonicClockImpl) Now(_ context.Context) Instant {
	return Instant(time.Since(programStart).Nanoseconds())
}

// Resolution returns the resolution of the monotonic clock.
func (i *monotonicClockImpl) Resolution(_ context.Context) Duration {
	// Go's time resolution is 1 nanosecond.
	return 1
}

// SubscribeInstant creates a pollable that resolves at a specific instant.
func (i *monotonicClockImpl) SubscribeInstant(_ context.Context, when Instant) Pollable {
	now := i.Now(context.Background())
	if when <= now {
		return i.pm.Add(io.ReadyPollable)
	}

	duration := time.Duration(when-now) * time.Nanosecond
	timer := time.NewTimer(duration)

	// 将 timer.Stop 作为取消函数
	p := io.NewPollable(func() { timer.Stop() })
	handle := i.pm.Add(p)

	go func() {
		<-timer.C
		p.SetReady()
	}()

	return handle
}

// SubscribeDuration creates a pollable that resolves after a duration.
func (i *monotonicClockImpl) SubscribeDuration(_ context.Context, when Duration) Pollable {
	if when == 0 {
		return i.pm.Add(io.ReadyPollable)
	}

	duration := time.Duration(when) * time.Nanosecond
	timer := time.NewTimer(duration)

	// 将 timer.Stop 作为取消函数
	p := io.NewPollable(func() { timer.Stop() })
	handle := i.pm.Add(p)

	go func() {
		<-timer.C
		p.SetReady()
	}()

	return handle
}
