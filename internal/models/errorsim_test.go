package models

import (
	"testing"
)

func TestUnmarshalErrorSim(t *testing.T) {
	if _, err := UnmarshalErrorSim(""); err != nil {
		t.Fatalf("empty input should not error, got %v", err)
	}
	if sim, err := UnmarshalErrorSim("  "); err != nil || sim != nil {
		t.Fatalf("blank input should yield no sim, got %+v err=%v", sim, err)
	}

	sim, err := UnmarshalErrorSim(`{"status":500,"failure_rate":30}`)
	if err != nil {
		t.Fatalf("decode valid config: %v", err)
	}
	if sim.Status != 500 || sim.FailureRate != 30 {
		t.Errorf("unexpected sim: %+v", sim)
	}

	if _, err := UnmarshalErrorSim("not json"); err == nil {
		t.Fatal("expected an error for invalid json")
	}
}

func TestErrorSimulationShouldFail(t *testing.T) {
	cases := []struct {
		name  string
		rate  int
		rolls []int
		want  bool
	}{
		{"unset always fails", 0, []int{0, 50, 99}, true},
		{"full rate always fails", 100, []int{0, 50, 99}, true},
		{"fifty percent", 50, []int{0, 49}, true},
		{"fifty percent miss", 50, []int{50, 99}, false},
		{"one percent", 1, []int{0}, true},
		{"one percent miss", 1, []int{1, 99}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sim := &ErrorSimulation{FailureRate: tc.rate}
			for _, roll := range tc.rolls {
				if got := sim.ShouldFail(roll); got != tc.want {
					t.Errorf("ShouldFail(%d) = %v, want %v", roll, got, tc.want)
				}
			}
		})
	}
}
