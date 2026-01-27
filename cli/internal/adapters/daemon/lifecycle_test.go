package daemon_test

import (
	"testing"
	"testing/synctest"
	"time"

	"go.trai.ch/same/internal/adapters/daemon"
)

func TestLifecycle_AutoShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timeout := 100 * time.Millisecond
		lc := daemon.NewLifecycle(timeout)

		select {
		case <-lc.ShutdownChan():
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected shutdown to be triggered")
		}
		synctest.Wait()
	})
}

func TestLifecycle_ResetPreventsShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timeout := 100 * time.Millisecond
		lc := daemon.NewLifecycle(timeout)

		time.Sleep(50 * time.Millisecond)
		lc.ResetTimer()

		select {
		case <-lc.ShutdownChan():
			t.Fatal("shutdown should not have triggered yet")
		case <-time.After(60 * time.Millisecond):
		}
		synctest.Wait()
	})
}

func TestLifecycle_IdleRemaining(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timeout := 100 * time.Millisecond
		lc := daemon.NewLifecycle(timeout)

		remaining := lc.IdleRemaining()
		if remaining > timeout {
			t.Fatalf("idle remaining %v > timeout %v", remaining, timeout)
		}

		time.Sleep(50 * time.Millisecond)
		remainingAfter := lc.IdleRemaining()

		if remainingAfter >= remaining {
			t.Fatalf("idle remaining should have decreased")
		}
		synctest.Wait()
	})
}

func TestLifecycle_Uptime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lc := daemon.NewLifecycle(1 * time.Hour)

		time.Sleep(10 * time.Millisecond)
		uptime := lc.Uptime()

		if uptime < 10*time.Millisecond {
			t.Fatalf("uptime %v < 10ms", uptime)
		}
		synctest.Wait()
	})
}

func TestLifecycle_LastActivity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lc := daemon.NewLifecycle(1 * time.Hour)

		initialActivity := lc.LastActivity()
		if initialActivity.IsZero() {
			t.Fatal("last activity should be set")
		}

		time.Sleep(10 * time.Millisecond)
		lc.ResetTimer()

		resetActivity := lc.LastActivity()
		if !resetActivity.After(initialActivity) {
			t.Fatal("last activity should have been updated")
		}
		synctest.Wait()
	})
}

func TestLifecycle_Shutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lc := daemon.NewLifecycle(1 * time.Hour)

		select {
		case <-lc.ShutdownChan():
			t.Fatal("should not have shutdown yet")
		case <-time.After(10 * time.Millisecond):
		}

		lc.Shutdown()

		select {
		case <-lc.ShutdownChan():
		case <-time.After(10 * time.Millisecond):
			t.Fatal("should have shutdown after calling Shutdown()")
		}
		synctest.Wait()
	})
}
