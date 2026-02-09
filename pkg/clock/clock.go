// Package clock implements a Lamport logical clock.
//
// From Lamport (1978), two implementation rules govern the clock:
//
//	IR1 (internal event): Before any internal event, increment the clock.
//	IR2 (message receipt): On receiving a message with timestamp t,
//	     set the clock to max(own, t) + 1.
//
// The total order function TotalOrderLess breaks ties deterministically
// using agent IDs, giving every participant the same ordering without
// coordination.
//
// Note: Clock is not goroutine-safe. In this architecture each Clock
// instance is short-lived (created per CLI invocation, seeded from the
// database). Cross-process coordination is handled by SQLite.
package clock

// Clock is a Lamport logical clock. Not goroutine-safe; see package doc.
type Clock struct {
	ts int64
}

// Tick implements IR1: increment the clock before an internal event.
// Returns the new timestamp.
func (c *Clock) Tick() int64 {
	c.ts++
	return c.ts
}

// Receive implements IR2: on receiving a message with timestamp received,
// set the clock to max(own, received) + 1. Returns the new timestamp.
func (c *Clock) Receive(received int64) int64 {
	if received > c.ts {
		c.ts = received
	}
	c.ts++
	return c.ts
}

// Value returns the current clock value without advancing it.
func (c *Clock) Value() int64 { return c.ts }

// Set initializes the clock to a specific value. Used to seed from the
// database at the start of a CLI invocation.
func (c *Clock) Set(v int64) { c.ts = v }

// TotalOrderLess defines a deterministic total order over events.
// Given two events with timestamps tsA and tsB from agents agentA and
// agentB, event A is "less" (has priority) if:
//
//	tsA < tsB, or
//	tsA == tsB and agentA < agentB (lexicographic)
//
// This is the standard Lamport total order used for mutual exclusion.
func TotalOrderLess(tsA int64, agentA string, tsB int64, agentB string) bool {
	if tsA != tsB {
		return tsA < tsB
	}
	return agentA < agentB
}
