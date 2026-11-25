package app

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeyedThrottleRunsOncePerBurst(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		th := &keyedThrottle{interval: diagnosticsPublishInterval, pending: map[string]bool{}}
		var gopls, other atomic.Int32
		for range 500 {
			th.trigger("gopls", func() { gopls.Add(1) })
		}
		th.trigger("other", func() { other.Add(1) })

		time.Sleep(diagnosticsPublishInterval)
		synctest.Wait()
		require.EqualValues(t, 1, gopls.Load(), "a burst runs once")
		require.EqualValues(t, 1, other.Load(), "keys are throttled independently")

		th.trigger("gopls", func() { gopls.Add(1) })
		time.Sleep(diagnosticsPublishInterval)
		synctest.Wait()
		require.EqualValues(t, 2, gopls.Load(), "a later change runs again")
	})
}
