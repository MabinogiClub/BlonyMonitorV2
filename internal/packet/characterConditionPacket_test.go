package packet

import "testing"

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
