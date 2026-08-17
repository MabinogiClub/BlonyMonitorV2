package app

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFarmSoundSpecialTakesPriorityOverPendingReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	played := make(chan string, 3)
	player := newFarmSoundPlayerWithPlayback(ctx, func() int { return 75 }, 15*time.Millisecond, func(filename string, volume int) time.Duration {
		if volume != 75 {
			t.Errorf("volume = %d, want 75", volume)
		}
		played <- filename
		return 35 * time.Millisecond
	})

	player.NotifyReady()
	player.NotifySpecial()
	if got := waitForFarmSound(t, played); got != "这是一颗高级种子.wav" {
		t.Fatalf("first sound = %q, want special seed", got)
	}
	assertNoFarmSound(t, played, 25*time.Millisecond)
	if got := waitForFarmSound(t, played); got != "农作物成熟.wav" {
		t.Fatalf("second sound = %q, want mature crop", got)
	}
}

func TestFarmSoundReadyCannotInterruptSpecial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	played := make(chan string, 3)
	player := newFarmSoundPlayerWithPlayback(ctx, func() int { return 100 }, 10*time.Millisecond, func(filename string, _ int) time.Duration {
		played <- filename
		return 40 * time.Millisecond
	})

	player.NotifySpecial()
	if got := waitForFarmSound(t, played); got != "这是一颗高级种子.wav" {
		t.Fatalf("first sound = %q, want special seed", got)
	}
	player.NotifyReady()
	assertNoFarmSound(t, played, 30*time.Millisecond)
	if got := waitForFarmSound(t, played); got != "农作物成熟.wav" {
		t.Fatalf("second sound = %q, want mature crop", got)
	}
}

func TestFarmSoundSpecialPreemptsAndRequeuesReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	played := make(chan string, 4)
	var mu sync.Mutex
	durations := map[string]time.Duration{
		"农作物成熟.wav":    60 * time.Millisecond,
		"这是一颗高级种子.wav": 30 * time.Millisecond,
	}
	player := newFarmSoundPlayerWithPlayback(ctx, func() int { return 100 }, 10*time.Millisecond, func(filename string, _ int) time.Duration {
		mu.Lock()
		duration := durations[filename]
		mu.Unlock()
		played <- filename
		return duration
	})

	player.NotifyReady()
	if got := waitForFarmSound(t, played); got != "农作物成熟.wav" {
		t.Fatalf("first sound = %q, want mature crop", got)
	}
	player.NotifySpecial()
	if got := waitForFarmSound(t, played); got != "这是一颗高级种子.wav" {
		t.Fatalf("preempting sound = %q, want special seed", got)
	}
	if got := waitForFarmSound(t, played); got != "农作物成熟.wav" {
		t.Fatalf("replayed sound = %q, want mature crop", got)
	}
}

func waitForFarmSound(t *testing.T, played <-chan string) string {
	t.Helper()
	select {
	case filename := <-played:
		return filename
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for farm sound")
		return ""
	}
}

func assertNoFarmSound(t *testing.T, played <-chan string, duration time.Duration) {
	t.Helper()
	select {
	case filename := <-played:
		t.Fatalf("unexpected sound during protected playback: %q", filename)
	case <-time.After(duration):
	}
}
