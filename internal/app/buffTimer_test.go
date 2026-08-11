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
