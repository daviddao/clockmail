package clock

import "testing"

func TestTickMonotonicallyIncreases(t *testing.T) {
	var c Clock
	prev := c.Value()
	for i := 0; i < 100; i++ {
		ts := c.Tick()
		if ts <= prev {
			t.Fatalf("Tick %d: got %d, want > %d", i, ts, prev)
		}
		prev = ts
	}
}

func TestTickStartsFromZero(t *testing.T) {
	var c Clock
	if v := c.Value(); v != 0 {
		t.Fatalf("new clock: got %d, want 0", v)
	}
	if ts := c.Tick(); ts != 1 {
		t.Fatalf("first Tick: got %d, want 1", ts)
	}
}

func TestReceiveMaxPlusOne(t *testing.T) {
	var c Clock
	c.Set(5)

	// Receive a higher timestamp: should set to max(5, 10)+1 = 11
	ts := c.Receive(10)
	if ts != 11 {
		t.Fatalf("Receive(10) from 5: got %d, want 11", ts)
	}

	// Receive a lower timestamp: should set to max(11, 3)+1 = 12
	ts = c.Receive(3)
	if ts != 12 {
		t.Fatalf("Receive(3) from 11: got %d, want 12", ts)
	}
}

func TestReceiveEqualTimestamp(t *testing.T) {
	var c Clock
	c.Set(10)
	ts := c.Receive(10)
	if ts != 11 {
		t.Fatalf("Receive(10) from 10: got %d, want 11", ts)
	}
}

func TestSetAndValue(t *testing.T) {
	var c Clock
	c.Set(42)
	if v := c.Value(); v != 42 {
		t.Fatalf("after Set(42): got %d, want 42", v)
	}
}

func TestSetThenTick(t *testing.T) {
	var c Clock
	c.Set(100)
	ts := c.Tick()
	if ts != 101 {
		t.Fatalf("Tick after Set(100): got %d, want 101", ts)
	}
}

func TestTotalOrderLess_DifferentTimestamps(t *testing.T) {
	if !TotalOrderLess(1, "b", 2, "a") {
		t.Fatal("expected (1,b) < (2,a)")
	}
	if TotalOrderLess(2, "a", 1, "b") {
		t.Fatal("expected (2,a) NOT < (1,b)")
	}
}

func TestTotalOrderLess_SameTimestamp_TieBreakByAgent(t *testing.T) {
	if !TotalOrderLess(5, "alice", 5, "bob") {
		t.Fatal("expected (5,alice) < (5,bob)")
	}
	if TotalOrderLess(5, "bob", 5, "alice") {
		t.Fatal("expected (5,bob) NOT < (5,alice)")
	}
}

func TestTotalOrderLess_Equal(t *testing.T) {
	if TotalOrderLess(5, "alice", 5, "alice") {
		t.Fatal("expected (5,alice) NOT < (5,alice) — strict less")
	}
}

func TestTotalOrderLess_Transitivity(t *testing.T) {
	// a < b < c => a < c
	a := TotalOrderLess(1, "x", 2, "x")
	b := TotalOrderLess(2, "x", 3, "x")
	c := TotalOrderLess(1, "x", 3, "x")
	if !a || !b || !c {
		t.Fatal("transitivity violated")
	}
}
