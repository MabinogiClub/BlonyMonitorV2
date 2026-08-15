package app

import (
	"context"
	"slices"
	"testing"
)

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
