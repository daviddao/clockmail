package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daviddao/clockmail/internal/clock"
	"github.com/daviddao/clockmail/internal/store"
)

// app holds shared state for all CLI subcommands.
type app struct {
	store   *store.Store
	agentID string // default agent from CLOCKMAIL_AGENT
}

// newApp opens the database and resolves the default agent identity.
func newApp() (*app, error) {
	dbPath := envOr("CLOCKMAIL_DB", "clockmail.db")
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

// printJSON writes v to stdout as indented JSON.
func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
