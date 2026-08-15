package app

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestPrepareWavPlaybackScalesPCM16Samples(t *testing.T) {
	wav := make([]byte, 48)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], 40)
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], 1)
	binary.LittleEndian.PutUint32(wav[24:28], 2)
	binary.LittleEndian.PutUint32(wav[28:32], 4)
	binary.LittleEndian.PutUint16(wav[32:34], 2)
	binary.LittleEndian.PutUint16(wav[34:36], 16)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], 4)
	binary.LittleEndian.PutUint16(wav[44:46], uint16(int16(10_000)))
	negativeSample := int16(-10_000)
	binary.LittleEndian.PutUint16(wav[46:48], uint16(negativeSample))

	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatalf("write WAV: %v", err)
	}

	scaled, duration, err := prepareWavPlayback(path, 50)
	if err != nil {
		t.Fatalf("prepare WAV playback: %v", err)
	}
	if duration != time.Second {
		t.Fatalf("duration = %v, want 1s", duration)
	}
	if got := int16(binary.LittleEndian.Uint16(scaled[44:46])); got != 5_000 {
		t.Fatalf("positive sample = %d, want 5000", got)
	}
	if got := int16(binary.LittleEndian.Uint16(scaled[46:48])); got != -5_000 {
		t.Fatalf("negative sample = %d, want -5000", got)
	}
}

func TestBuffTimerManagerIncludesSuperBurningBuff(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := NewBuffTimerManager(context.Background(), "")

	if got := mgr.targetBuffs[1225]; got != "超燃咚咚" {
		t.Fatalf("buff 1225 name = %q, want %q", got, "超燃咚咚")
	}
	if got := mgr.getNotifyThreshold(1225); got != 30 {
		t.Fatalf("buff 1225 notify threshold = %d, want 30", got)
	}
	if got := resolveBuffDuration(1225, 0); got != 180 {
		t.Fatalf("buff 1225 fallback duration = %d, want 180", got)
	}
	if got := resolveBuffDuration(1225, 175); got != 175 {
		t.Fatalf("buff 1225 packet duration = %d, want 175", got)
	}
	if !slices.Contains(mgr.buffOrder, uint32(1225)) {
		t.Fatalf("buff 1225 missing from default order: %v", mgr.buffOrder)
	}
	for _, buff := range []struct {
		id   uint32
		name string
	}{
		{63, "攻击力增加"},
		{1121, "魔法攻击强化"},
		{1150, "炼金术伤害增加"},
	} {
		if got := mgr.targetBuffs[buff.id]; got != buff.name {
			t.Errorf("buff %d name = %q, want %q", buff.id, got, buff.name)
		}
		if got := mgr.getNotifyThreshold(buff.id); got != 30 {
			t.Errorf("buff %d notify threshold = %d, want 30", buff.id, got)
		}
		if got := resolveBuffDuration(buff.id, 0); got != 1800 {
			t.Errorf("buff %d fallback duration = %d, want 1800", buff.id, got)
		}
		if !slices.Contains(mgr.buffOrder, buff.id) {
			t.Errorf("buff %d missing from default order: %v", buff.id, mgr.buffOrder)
		}
	}
}

func TestStartTimerIgnoresConditionUpdateWithSameDisableAt(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := NewBuffTimerManager(ctx, "1")

	mgr.StartTimer(515, 1, "Self", 63922387254713, 300)
	first := mgr.timers["1_515"]
	if first == nil {
		t.Fatal("expected status support timer to start")
	}

	mgr.StartTimer(515, 1, "Self", 63922387254713, 300)
	if got := mgr.timers["1_515"]; got != first {
		t.Fatal("same buff instance must not restart the timer")
	}
}

func TestStartTimerRestartsWhenDisableAtChanges(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := NewBuffTimerManager(ctx, "1")

	mgr.StartTimer(515, 1, "Self", 63922387254713, 300)
	first := mgr.timers["1_515"]
	mgr.StartTimer(515, 1, "Self", 63922387554713, 300)
	second := mgr.timers["1_515"]

	if second == nil || second == first {
		t.Fatal("new buff instance must restart the timer")
	}
	if second.DisableAt != 63922387554713 {
		t.Fatalf("disableAt = %d, want %d", second.DisableAt, int64(63922387554713))
	}
}
