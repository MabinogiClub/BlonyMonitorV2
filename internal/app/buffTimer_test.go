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

func TestBuffTimerManagerIncludesBossOnlyHangedManBuff(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := NewBuffTimerManager(context.Background(), "1")

	if got := mgr.targetBuffs[1026]; got != "倒吊者" {
		t.Fatalf("buff 1026 name = %q, want %q", got, "倒吊者")
	}
	if got := mgr.getNotifyThreshold(1026); got != 10 {
		t.Fatalf("buff 1026 notify threshold = %d, want 10", got)
	}
	if _, ok := targetOnlyNotificationBuffs[1026]; !ok {
		t.Fatal("buff 1026 must be marked as target-only")
	}
	if !slices.Contains(mgr.buffOrder, uint32(1026)) {
		t.Fatalf("buff 1026 missing from default order: %v", mgr.buffOrder)
	}
}

func TestStartTimerAllowsHangedManOnlyForMarkedBoss(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := NewBuffTimerManager(ctx, "1")

	mgr.StartTimer(1026, 2, "枯木之佩塔克", 63922387254713, 300)
	if mgr.timers["2_1026"] == nil {
		t.Fatal("marked boss hanged man timer did not start")
	}
	mgr.StartTimer(1026, 3, "普通怪物", 63922387254713, 300)
	if mgr.timers["3_1026"] != nil {
		t.Fatal("hanged man timer started for an unmarked target")
	}
	mgr.StartTimer(1026, 1, "Flandre", 63922387254713, 300)
	if mgr.timers["1_1026"] != nil {
		t.Fatal("boss-only hanged man timer started for the player")
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

func TestAdjustRepeatedBuffDurationDetectsHangedManDoubleTime(t *testing.T) {
	// The final capture activation advanced SBT by 63.565s while only 10.566s
	// elapsed locally, leaving an extra 53s to add to the base duration.
	got := adjustRepeatedBuffDuration(1026, 53, 63924159538884, 63924159602449, 10*time.Second+566*time.Millisecond)
	if got != 106 {
		t.Fatalf("duration = %d, want 106", got)
	}
}

func TestAdjustRepeatedBuffDurationIgnoresNormalRefresh(t *testing.T) {
	got := adjustRepeatedBuffDuration(1026, 53, 63924159528307, 63924159538884, 10*time.Second+577*time.Millisecond)
	if got != 53 {
		t.Fatalf("duration = %d, want 53", got)
	}
	if got := adjustRepeatedBuffDuration(515, 300, 1000, 70000, time.Second); got != 300 {
		t.Fatalf("non-hanged-man duration = %d, want 300", got)
	}
}

func TestBuffTimerManagerIncludesManusEnergyDrink(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := NewBuffTimerManager(context.Background(), "1")

	if got := mgr.targetBuffs[835]; got != "马纽斯功能饮料" {
		t.Fatalf("buff 835 name = %q, want %q", got, "马纽斯功能饮料")
	}
	if _, ok := targetOnlyNotificationBuffs[835]; ok {
		t.Fatal("buff 835 must be monitored on the player, not as a boss debuff")
	}
	if got := mgr.getNotifyThreshold(835); got != 30 {
		t.Fatalf("buff 835 notify threshold = %d, want 30", got)
	}
	// The normal drink lasts 10 minutes and the enhanced one 20 minutes; they
	// share condition 835, so the fallback has to cover the shorter one.
	if got := resolveBuffDuration(835, 0); got != 600 {
		t.Fatalf("buff 835 fallback duration = %d, want 600", got)
	}
	if got := resolveBuffDuration(835, 1200); got != 1200 {
		t.Fatalf("buff 835 packet duration = %d, want 1200", got)
	}
	if !slices.Contains(mgr.buffOrder, uint32(835)) {
		t.Fatalf("buff 835 missing from default order: %v", mgr.buffOrder)
	}
}

func TestBuffDisplayNameSeparatesManusEnergyDrinkVariants(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	mgr := NewBuffTimerManager(context.Background(), "1")

	cases := []struct {
		duration int64
		want     string
	}{
		{0, "马纽斯功能饮料"},
		{600, "马纽斯功能饮料"},
		{1200, "强化的马纽斯功能饮料"},
	}
	for _, testCase := range cases {
		if got := mgr.buffDisplayName(835, testCase.duration); got != testCase.want {
			t.Errorf("buff 835 name at %ds = %q, want %q", testCase.duration, got, testCase.want)
		}
		// Names without duration variants must stay untouched.
		if got := mgr.buffDisplayName(680, testCase.duration); got != "战争序曲" {
			t.Errorf("buff 680 name at %ds = %q, want %q", testCase.duration, got, "战争序曲")
		}
	}
}

func TestActiveTimerReportsEnhancedDrinkName(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := NewBuffTimerManager(ctx, "1")

	mgr.StartTimer(835, 1, "Self", 0, 1200)
	timer := mgr.timers["1_835"]
	if timer == nil {
		t.Fatal("energy drink timer did not start")
	}
	if timer.DurationSec != 1200 {
		t.Fatalf("duration = %d, want 1200", timer.DurationSec)
	}

	active := mgr.GetActiveTimersInfo()
	if len(active) != 1 || active[0].BuffName != "强化的马纽斯功能饮料" {
		t.Fatalf("active timers = %+v, want the enhanced drink name", active)
	}
	for _, buff := range mgr.GetBuffDisplayList() {
		if buff.CCId != 835 {
			continue
		}
		if buff.BuffName != "强化的马纽斯功能饮料" {
			t.Fatalf("display name = %q, want 强化的马纽斯功能饮料", buff.BuffName)
		}
		if !buff.IsActive || buff.TotalTime != 1200 {
			t.Fatalf("display state = %+v, want active 1200s", buff)
		}
	}
}

func TestStartTimerUsesFallbackDurationForDrink(t *testing.T) {
	t.Setenv("MABI_WORK_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := NewBuffTimerManager(ctx, "1")

	// A drink packet without DURA/MCAGT still has to start a timer instead of
	// silently refusing to run.
	mgr.StartTimer(835, 1, "Self", 0, 0)
	timer := mgr.timers["1_835"]
	if timer == nil {
		t.Fatal("energy drink timer did not start without a packet duration")
	}
	if timer.DurationSec != 600 {
		t.Fatalf("duration = %d, want 600", timer.DurationSec)
	}

	// It is a self buff, so conditions observed on other entities must not raise
	// the local character's prompt.
	mgr.StartTimer(835, 2, "Other", 0, 1200)
	if mgr.timers["2_835"] != nil {
		t.Fatal("energy drink timer started for another entity")
	}
}
