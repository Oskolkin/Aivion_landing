package runtime

import (
	"context"
	"sync/atomic"
	"time"

	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
)

type leaseHeartbeater interface {
	Heartbeat(ctx context.Context, l analysispg.Lease, expires time.Time) error
}

type heartbeatControl struct {
	cancel context.CancelFunc
	done   chan struct{}
	lost   atomic.Bool
}

func heartbeatInterval(lease time.Duration) time.Duration {
	if lease <= 0 {
		return 20 * time.Second
	}
	i := lease / 3
	if i > 20*time.Second {
		i = 20 * time.Second
	}
	if i < 10*time.Millisecond {
		i = 10 * time.Millisecond
	}
	return i
}

func startLeaseHeartbeat(
	parent context.Context,
	hb leaseHeartbeater,
	lease analysispg.Lease,
	leaseDur time.Duration,
	now func() time.Time,
	onLost func(),
) *heartbeatControl {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	ctl := &heartbeatControl{done: make(chan struct{})}
	interval := heartbeatInterval(leaseDur)
	hbCtx, cancel := context.WithCancel(parent)
	ctl.cancel = cancel
	go func() {
		defer close(ctl.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				callCtx, callCancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
				err := hb.Heartbeat(callCtx, lease, now().Add(leaseDur))
				callCancel()
				if err != nil {
					ctl.lost.Store(true)
					if onLost != nil {
						onLost()
					}
					return
				}
			}
		}
	}()
	return ctl
}

func (c *heartbeatControl) Stop() {
	if c == nil {
		return
	}
	c.cancel()
	<-c.done
}

func (c *heartbeatControl) Lost() bool {
	return c != nil && c.lost.Load()
}
