package app

import "testing"

func TestIsBossEntityReuseUsesPercentages(t *testing.T) {
	tests := []struct {
		name string
		prev float64
		curr float64
		want bool
	}{
		{name: "different max hp", prev: 5, curr: 95, want: true},
		{name: "not low enough", prev: 10, curr: 95, want: false},
		{name: "not full enough", prev: 5, curr: 90, want: true},
		{name: "normal damage", prev: 50, curr: 95, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBossEntityReuse(tt.prev, tt.curr); got != tt.want {
				t.Fatalf("isBossEntityReuse(%v, %v) = %v, want %v", tt.prev, tt.curr, got, tt.want)
			}
		})
	}
}
