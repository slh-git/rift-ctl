package cmd

import (
	"testing"
)

func TestParseIntRange(t *testing.T) {
	cases := []struct {
		in      string
		wantMin *int
		wantMax *int
		wantErr bool
	}{
		{"", nil, nil, false},
		{"3", intPtr(3), intPtr(3), false},
		{"2-4", intPtr(2), intPtr(4), false},
		{"2-", intPtr(2), nil, false},
		{"-3", nil, intPtr(3), false},
		{"-", nil, nil, true},
		{"4-2", nil, nil, true},
		{"x", nil, nil, true},
	}
	for _, tc := range cases {
		got, err := parseIntRange(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !intPtrEq(got.Min, tc.wantMin) || !intPtrEq(got.Max, tc.wantMax) {
			t.Fatalf("%q: got min=%v max=%v, want min=%v max=%v",
				tc.in, got.Min, got.Max, tc.wantMin, tc.wantMax)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" Fury, Calm , ,Mind ")
	want := []string{"Fury", "Calm", "Mind"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func intPtr(n int) *int { return &n }

func intPtrEq(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
