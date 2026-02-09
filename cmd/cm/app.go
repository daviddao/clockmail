package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daviddao/clockmail/pkg/clock"
	"github.com/daviddao/clockmail/pkg/model"
	"github.com/daviddao/clockmail/pkg/store"
)

// app holds shared state for all CLI subcommands.
type app struct {
	store   *store.Store
	agentID string // default agent from CLOCKMAIL_AGENT
}

// newApp opens the database and resolves the default agent identity.
// Creates the .clockmail/ directory if using the default DB path.
func newApp() (*app, error) {
	dbPath := envOr("CLOCKMAIL_DB", defaultDB)
	if dbPath == defaultDB {
		if err := os.MkdirAll(defaultDir, 0755); err != nil {
			return nil, fmt.Errorf("cannot create %s: %w", defaultDir, err)
		}
	}
	s, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database %q: %w", dbPath, err)
	}
	return &app{
		store:   s,
		agentID: envOr("CLOCKMAIL_AGENT", ""),
	}, nil
}

// Close releases the database connection.
func (a *app) Close() { a.store.Close() }

// resolveAgent returns the agent ID from the flag (if non-empty), falling
// back to the CLOCKMAIL_AGENT environment variable.
func (a *app) resolveAgent(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if a.agentID != "" {
		return a.agentID, nil
	}
	return "", fmt.Errorf("no agent ID: pass --agent or set CLOCKMAIL_AGENT")
}

// getClock returns a Lamport clock seeded from the agent's persisted value.
// Each agent owns its clock exclusively — no two processes should operate
// as the same agent concurrently, matching Lamport's model where each
// process has its own local clock.
func (a *app) getClock(agentID string) *clock.Clock {
	c := &clock.Clock{}
	if ag, err := a.store.GetAgent(agentID); err == nil {
		c.Set(ag.Clock)
	}
	return c
}

// resolveEpochRound returns the epoch/round to use. A flag value of -1
// (the sentinel) means "keep the agent's current value from the DB".
func (a *app) resolveEpochRound(agentID string, flagEpoch, flagRound int64) (int64, int64) {
	ep, rn := flagEpoch, flagRound
	if ep >= 0 && rn >= 0 {
		return ep, rn
	}
	if ag, err := a.store.GetAgent(agentID); err == nil {
		if ep < 0 {
			ep = ag.Epoch
		}
		if rn < 0 {
			rn = ag.Round
		}
	} else {
		if ep < 0 {
			ep = 0
		}
		if rn < 0 {
			rn = 0
		}
	}
	return ep, rn
}

// peekInbox checks for pending messages without advancing the cursor.
// Returns the messages and count. Used by commands that want to show
// pending messages as a side effect (send, lock, etc.).
func (a *app) peekInbox(agentID string) ([]model.Event, int) {
	if agentID == "" {
		return nil, 0
	}
	cursor := a.store.GetCursor(agentID)
	msgs, err := a.store.ListEventsForAgent(agentID, cursor, 100)
	if err != nil {
		return nil, 0
	}
	return msgs, len(msgs)
}

// drainInbox fetches pending messages, applies Lamport IR2, advances the
// cursor, and returns the messages. This is the "receive" side effect that
// send, lock, and other commands use to force bidirectional communication.
func (a *app) drainInbox(agentID string, c *clock.Clock) []model.Event {
	if agentID == "" {
		return nil
	}
	cursor := a.store.GetCursor(agentID)
	msgs, err := a.store.ListEventsForAgent(agentID, cursor, 100)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	var maxTS int64
	for _, e := range msgs {
		c.Receive(e.LamportTS)
		if e.LamportTS > maxTS {
			maxTS = e.LamportTS
		}
	}
	if ag, _ := a.store.GetAgent(agentID); ag != nil {
		_ = a.store.UpdateAgentClock(agentID, c.Value(), ag.Epoch, ag.Round)
	}
	if maxTS > 0 {
		_ = a.store.SetCursor(agentID, maxTS+1)
	}
	return msgs
}

// printInbox prints received messages to stderr so they don't interfere
// with the command's primary stdout output. Returns the count printed.
func printInbox(msgs []model.Event) int {
	if len(msgs) == 0 {
		return 0
	}
	fmt.Fprintf(os.Stderr, "\n=== %d pending message(s) ===\n", len(msgs))
	for _, e := range msgs {
		body := e.Body
		if len(body) > 120 {
			body = body[:120] + "..."
		}
		fmt.Fprintf(os.Stderr, "  [ts=%d] %s: %s\n", e.LamportTS, e.AgentID, body)
	}
	fmt.Fprintf(os.Stderr, "============================\n\n")
	return len(msgs)
}

// printJSON writes v to stdout as indented JSON.
func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
