package packet

import (
	"fmt"
	"testing"
	"time"
)

func TestParseConditionDetails(t *testing.T) {
	details := ParseConditionDetails("MCMBAMAX:f:50.531799;MCAGT:8:63921988401203;MCMBGA:b:false;SBT:8:63921988580203;")
	if got := details["MCMBAMAX"]; got.Type != "f" || got.Value != "50.531799" {
		t.Fatalf("unexpected MCMBAMAX: %+v", got)
	}
	if got := details["MCAGT"]; got.Type != "8" || got.Value != "63921988401203" {
		t.Fatalf("unexpected MCAGT: %+v", got)
	}
	if got := details["MCMBGA"]; got.Type != "b" || got.Value != "false" {
		t.Fatalf("unexpected MCMBGA: %+v", got)
	}
}

func TestExtractDurationSeconds(t *testing.T) {
	details := ParseConditionDetails("DURA:4:1800000;")
	if got := extractDurationSeconds(details); got != 1800 {
		t.Fatalf("duration = %d, want 1800", got)
	}
}

func TestDurationFromDisableAtForAttackPotion(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	tests := []struct {
		name      string
		packetAt  time.Time
		disableAt int64
		want      int64
	}{
		{
			name:      "30 minute potion",
			packetAt:  time.Date(2026, 8, 15, 10, 51, 19, 389_214_600, location),
			disableAt: 63922389669062,
			want:      1800,
		},
		{
			name:      "one hour potion",
			packetAt:  time.Date(2026, 8, 15, 10, 51, 29, 169_100_600, location),
			disableAt: 63922391478845,
			want:      3600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition, err := ParseCharacterConditionPacket(&GamePacket{
				At: tt.packetAt,
				Id: 1,
				Msg: Message{
					NewMessageElemByte(1),
					NewMessageElemInt(63),
					NewMessageElemLong(uint64(tt.disableAt)),
					NewMessageElemString(fmt.Sprintf("SBT:8:%d;", tt.disableAt)),
					NewMessageElemLong(0),
				},
			})
			if err != nil {
				t.Fatalf("parse condition: %v", err)
			}
			if condition.Duration != tt.want {
				t.Fatalf("duration = %d, want %d", condition.Duration, tt.want)
			}
		})
	}
}
