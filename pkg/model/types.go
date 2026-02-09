// Package model defines the core domain types for clockmail.
//
// Clockmail coordinates concurrent AI agent sessions using two ideas:
//
//   - Lamport clocks (1978): logical timestamps that establish causal ordering.
//     Every event gets a timestamp; messages carry timestamps; on receipt the
//     clock advances to max(own, received) + 1. Ties are broken by agent ID,
//     giving a deterministic total order with no central coordinator.
//
//   - Naiad frontiers (2013): structured timestamps (epoch, round) that let
//     independent work proceed without barriers. An agent can finalize work at
//     timestamp t only when no outstanding work anywhere could produce input
//     at t. The "frontier" is the antichain of earliest-incomplete pointstamps.
package model

import "time"

// Timestamp is a Naiad-style structured timestamp: (Epoch, Round).
// Epoch identifies a batch of work (a task, a PR, a feature).
// Round identifies a refinement iteration within that epoch.
type Timestamp struct {
	Epoch int64 `json:"epoch"`
	Round int64 `json:"round"`
}

// LessEq returns true if t <= other in the Naiad partial order.
// (e1,r1) <= (e2,r2) iff e1<=e2 AND r1<=r2.
func (t Timestamp) LessEq(other Timestamp) bool {
	return t.Epoch <= other.Epoch && t.Round <= other.Round
}

// Less returns true if t < other (strictly less in the partial order).
func (t Timestamp) Less(other Timestamp) bool {
	return t.LessEq(other) && t != other
}

// Pointstamp is a (Timestamp, AgentID) pair from Naiad. In our model the
// "location" dimension is the agent identity.
type Pointstamp struct {
	Timestamp Timestamp `json:"timestamp"`
	AgentID   string    `json:"agent_id"`
}

// EventKind enumerates the types of events in the append-only log.
type EventKind string

const (
	EventMsg        EventKind = "msg"
	EventLockReq    EventKind = "lock_req"
	EventLockRel    EventKind = "lock_rel"
	EventProgress   EventKind = "progress"
	EventReviewReq  EventKind = "review_req"
	EventReviewDone EventKind = "review_done"
)

// Agent represents a registered agent session.
type Agent struct {
	ID         string    `json:"id"`
	Clock      int64     `json:"clock"`
	Epoch      int64     `json:"epoch"`
	Round      int64     `json:"round"`
	Registered time.Time `json:"registered_at"`
	LastSeen   time.Time `json:"last_seen_at"`
}

// Event is a single entry in the append-only event log.
type Event struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	LamportTS int64     `json:"lamport_ts"`
	Epoch     int64     `json:"epoch"`
	Round     int64     `json:"round"`
	Kind      EventKind `json:"kind"`
	Target    string    `json:"target,omitempty"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Lock represents an active file reservation.
type Lock struct {
	Path      string    `json:"path"`
	AgentID   string    `json:"agent_id"`
	LamportTS int64     `json:"lamport_ts"`
	Epoch     int64     `json:"epoch"`
	Exclusive bool      `json:"exclusive"`
	ExpiresAt time.Time `json:"expires_at"`
}
