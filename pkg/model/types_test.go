package model

import "testing"

func TestTimestamp_LessEq_Reflexive(t *testing.T) {
	ts := Timestamp{Epoch: 3, Round: 5}
	if !ts.LessEq(ts) {
		t.Fatal("LessEq should be reflexive")
	}
}

func TestTimestamp_Less_NotReflexive(t *testing.T) {
	ts := Timestamp{Epoch: 3, Round: 5}
	if ts.Less(ts) {
		t.Fatal("Less should NOT be reflexive")
	}
}

func TestTimestamp_LessEq_PartialOrder(t *testing.T) {
	cases := []struct {
		name   string
		a, b   Timestamp
		expect bool
	}{
		{"both less", Timestamp{1, 1}, Timestamp{2, 2}, true},
		{"equal", Timestamp{2, 2}, Timestamp{2, 2}, true},
		{"epoch less, round equal", Timestamp{1, 2}, Timestamp{2, 2}, true},
		{"epoch equal, round less", Timestamp{2, 1}, Timestamp{2, 2}, true},
		{"epoch greater", Timestamp{3, 1}, Timestamp{2, 2}, false},
		{"round greater", Timestamp{1, 3}, Timestamp{2, 2}, false},
		{"incomparable: epoch less, round greater", Timestamp{1, 3}, Timestamp{2, 2}, false},
		{"incomparable: epoch greater, round less", Timestamp{3, 1}, Timestamp{2, 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.a.LessEq(tc.b)
			if got != tc.expect {
				t.Fatalf("%v.LessEq(%v) = %v, want %v", tc.a, tc.b, got, tc.expect)
			}
		})
	}
}

func TestTimestamp_Less_StrictlyLess(t *testing.T) {
	cases := []struct {
		name   string
		a, b   Timestamp
		expect bool
	}{
		{"strictly less both", Timestamp{1, 1}, Timestamp{2, 2}, true},
		{"equal — not strict", Timestamp{2, 2}, Timestamp{2, 2}, false},
		{"epoch less, round equal — strict", Timestamp{1, 2}, Timestamp{2, 2}, true},
		{"incomparable", Timestamp{1, 3}, Timestamp{2, 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.a.Less(tc.b)
			if got != tc.expect {
				t.Fatalf("%v.Less(%v) = %v, want %v", tc.a, tc.b, got, tc.expect)
			}
		})
	}
}

func TestTimestamp_Antisymmetric(t *testing.T) {
	a := Timestamp{1, 2}
	b := Timestamp{2, 1}
	// Incomparable: neither a <= b nor b <= a
	if a.LessEq(b) {
		t.Fatal("a should not be LessEq b")
	}
	if b.LessEq(a) {
		t.Fatal("b should not be LessEq a")
	}
}

func TestTimestamp_Transitive(t *testing.T) {
	a := Timestamp{1, 1}
	b := Timestamp{2, 2}
	c := Timestamp{3, 3}
	if !a.Less(b) || !b.Less(c) {
		t.Fatal("precondition failed")
	}
	if !a.Less(c) {
		t.Fatal("transitivity: a < b < c but not a < c")
	}
}

func TestTimestamp_ZeroValue(t *testing.T) {
	zero := Timestamp{}
	any := Timestamp{1, 0}
	if !zero.LessEq(any) {
		t.Fatal("zero should be <= any non-negative timestamp")
	}
}
