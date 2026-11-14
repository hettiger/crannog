package crannog

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/hettiger/crannog/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockedReadinessRuntime struct {
	*fakeRuntime
	block string
	proxy string
}

func (r *blockedReadinessRuntime) Exec(ctx context.Context, name string, o runtime.ExecOptions, args ...string) ([]byte, error) {
	if r.block == "status" {
		return nil, errors.New("not ready")
	}
	if (r.block == "proxy") == (name == r.proxy) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return r.fakeRuntime.Exec(ctx, name, o, args...)
}

func (r *blockedReadinessRuntime) List(ctx context.Context, prefix string) ([]runtime.Container, error) {
	if r.block == "status" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return r.fakeRuntime.List(ctx, prefix)
}

func TestStartReadinessDeadline(t *testing.T) {
	for _, block := range []string{"proxy", "crannog", "status"} {
		t.Run(block, func(t *testing.T) {
			f := newFixture(t)
			tg := f.existing("main", defaultSetup(t))
			f.m.Runtime = &blockedReadinessRuntime{fakeRuntime: f.rt, block: block, proxy: tg.names.Proxy}
			// Use the production deadline without spending a minute per case.
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				err := f.m.start(tg, status{})
				require.ErrorContains(t, err, "timed out after 1m0s")
				assert.Equal(t, commandTimeout, time.Since(started))
				if block == "crannog" {
					assert.Contains(t, err.Error(), readyMarker)
				}
			})
		})
	}
}

// The commands before a session have the same deadline as the probes: a
// script that never returns in the crannog does not hang checkout.
func TestSessionCommandsDeadline(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	for _, c := range []string{tg.names.Container, tg.names.Proxy} {
		f.rt.containers[c].State = runtime.StateRunning
	}
	f.m.Runtime = &blockedReadinessRuntime{fakeRuntime: f.rt, block: "crannog", proxy: tg.names.Proxy}
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		err := f.m.prepareSession(tg)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.ErrorContains(t, err, "writing /etc/hosts")
		assert.Equal(t, commandTimeout, time.Since(started))
	})
}

func TestReadinessStopsPollingStoppedContainer(t *testing.T) {
	f := newFixture(t)
	synctest.Test(t, func(t *testing.T) {
		err := f.m.waitFor("missing", "readiness", func(context.Context) bool { return false })
		require.ErrorContains(t, err, "stopped while waiting")
	})
}
