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
}
