package breaker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

const Inf = math.MaxFloat64

type Limiter struct {
	limit float64
	burst int

	mu     sync.Mutex
	tokens float64

	last time.Time

	lastEvent time.Time
}

func (lim *Limiter) Limit() float64 {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	return lim.limit
}

func (lim *Limiter) Burst() int {
	return lim.burst
}

func NewLimiter(limit float64, burst int) *Limiter {
	return &Limiter{
		limit: limit,
		burst: burst,
	}
}

func (lim *Limiter) Allow() bool {
	return lim.AllowN(time.Now(), 1)
}

func (lim *Limiter) AllowN(now time.Time, n int) bool {
	return lim.reserveN(now, n, 0).ok
}

type Reservation struct {
	ok        bool
	lim       *Limiter
	tokens    int
	timeToAct time.Time
	// 保留期内的 limit
	limit float64
}

func (r *Reservation) OK() bool {
	return r.ok
}

func (r *Reservation) Delay() time.Duration {
	return r.DelayFrom(time.Now())
}

const InfDuration = time.Duration(1<<63 - 1)

func (r *Reservation) DelayFrom(now time.Time) time.Duration {
	if !r.ok {
		return InfDuration
	}
	delay := r.timeToAct.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

func (r *Reservation) Cancel() {
	r.CancelAt(time.Now())
	return
}

func (r *Reservation) CancelAt(now time.Time) {
	if !r.ok {
		return
	}

	r.lim.mu.Lock()
	defer r.lim.mu.Unlock()

	if r.lim.limit == Inf || r.tokens == 0 || r.timeToAct.Before(now) {
		return
	}

	restoreTokens := float64(r.tokens) - tokensFromDuration(r.lim.lastEvent.Sub(r.timeToAct), r.limit)
	if restoreTokens <= 0 {
		return
	}
	now, _, tokens := r.lim.advance(now)
	tokens += restoreTokens
	if burst := float64(r.lim.burst); tokens > burst {
		tokens = burst
	}
	r.lim.last = now
	r.lim.tokens = tokens
	if r.timeToAct == r.lim.lastEvent {
		prevEvent := r.timeToAct.Add(durationFromTokens(float64(-r.tokens), r.limit))
		if !prevEvent.Before(now) {
			r.lim.lastEvent = prevEvent
		}
	}

	return
}

func (lim *Limiter) Reserve() *Reservation {
	return lim.ReserveN(time.Now(), 1)
}
func (lim *Limiter) ReserveN(now time.Time, n int) *Reservation {
	r := lim.reserveN(now, n, InfDuration)
	return &r
}

func (lim *Limiter) Wait(ctx context.Context) (err error) {
	return lim.WaitN(ctx, 1)
}

func (lim *Limiter) WaitN(ctx context.Context, n int) (err error) {
	if n > lim.burst && lim.limit != Inf {
		return fmt.Errorf("rate: Wait(n=%d) exceeds limiter's burst %d", n, lim.burst)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	now := time.Now()
	waitLimit := InfDuration
	if deadline, ok := ctx.Deadline(); ok {
		waitLimit = deadline.Sub(now)
	}
	r := lim.reserveN(now, n, waitLimit)
	if !r.ok {
		return fmt.Errorf("rate: Wait(n=%d) would exceed context deadline", n)
	}
	delay := r.DelayFrom(now)
	if delay == 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		r.Cancel()
		return ctx.Err()
	}
}

func (lim *Limiter) SetLimit(newLimit float64) {
	lim.SetLimitAt(time.Now(), newLimit)
}

func (lim *Limiter) SetLimitAt(now time.Time, newLimit float64) {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	now, _, tokens := lim.advance(now)

	lim.last = now
	lim.tokens = tokens
	lim.limit = newLimit
}

func (lim *Limiter) reserveN(now time.Time, n int, maxFutureReserve time.Duration) Reservation {
	lim.mu.Lock()

	if lim.limit == Inf {
		lim.mu.Unlock()
		return Reservation{
			ok:        true,
			lim:       lim,
			tokens:    n,
			timeToAct: now,
		}
	}

	now, last, tokens := lim.advance(now)

	tokens -= float64(n)

	var waitDuration time.Duration
	if tokens < 0 {
		waitDuration = durationFromTokens(-tokens, lim.limit)
	}

	ok := n <= lim.burst && waitDuration <= maxFutureReserve

	r := Reservation{
		ok:    ok,
		lim:   lim,
		limit: lim.limit,
	}
	if ok {
		r.tokens = n
		r.timeToAct = now.Add(waitDuration)
	}

	if ok {
		lim.last = now
		lim.tokens = tokens
		lim.lastEvent = r.timeToAct
	} else {
		lim.last = last
	}

	lim.mu.Unlock()
	return r
}

func (lim *Limiter) advance(now time.Time) (newNow time.Time, newLast time.Time, newTokens float64) {
	last := lim.last
	if now.Before(last) {
		last = now
	}

	maxElapsed := durationFromTokens(float64(lim.burst)-lim.tokens, lim.limit)
	elapsed := now.Sub(last)
	if elapsed > maxElapsed {
		elapsed = maxElapsed
	}

	delta := tokensFromDuration(elapsed, lim.limit)
	tokens := lim.tokens + delta
	if burst := float64(lim.burst); tokens > burst {
		tokens = burst
	}

	return now, last, tokens
}

func durationFromTokens(tokens, limit float64) time.Duration {
	seconds := tokens / float64(limit)
	return time.Nanosecond * time.Duration(1e9*seconds)
}

func tokensFromDuration(d time.Duration, limit float64) float64 {
	return d.Seconds() * float64(limit)
}
