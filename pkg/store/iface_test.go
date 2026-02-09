package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

// TestStoreImplementsInterface verifies at runtime that *Store satisfies
// StoreInterface by calling every method on a real store.
func TestStoreImplementsInterface(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Use the interface type to verify all methods are callable.
	var iface StoreInterface = s

	// Agents
	ag, err := iface.RegisterAgent("test-agent")
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if ag.ID != "test-agent" {
		t.Errorf("expected agent ID 'test-agent', got %q", ag.ID)
	}

	ag2, err := iface.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if ag2.ID != "test-agent" {
		t.Errorf("GetAgent returned wrong ID: %q", ag2.ID)
	}

	if err := iface.UpdateAgentClock("test-agent", 5, 1, 0); err != nil {
		t.Fatalf("UpdateAgentClock: %v", err)
	}

	agents, err := iface.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	// Cursors
	cur := iface.GetCursor("test-agent")
	if cur != 0 {
		t.Errorf("expected cursor 0, got %d", cur)
	}
	if err := iface.SetCursor("test-agent", 42); err != nil {
		t.Fatalf("SetCursor: %v", err)
	}
	cur = iface.GetCursor("test-agent")
	if cur != 42 {
		t.Errorf("expected cursor 42, got %d", cur)
	}

	// Events
	e := &model.Event{
		AgentID:   "test-agent",
		LamportTS: 1,
		Kind:      model.EventMsg,
		Target:    "other",
		Body:      "hello",
		CreatedAt: time.Now(),
	}
	id, err := iface.InsertEvent(e)
	if err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive event ID, got %d", id)
	}

	events, err := iface.ListEvents(0, 10)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	events2, err := iface.ListEventsSinceID(0, 10)
	if err != nil {
		t.Fatalf("ListEventsSinceID: %v", err)
	}
	if len(events2) != 1 {
		t.Errorf("expected 1 event, got %d", len(events2))
	}

	maxID := iface.MaxEventID()
	if maxID != 1 {
		t.Errorf("expected MaxEventID=1, got %d", maxID)
	}

	count := iface.CountEvents()
	if count != 1 {
		t.Errorf("expected CountEvents=1, got %d", count)
	}

	agentEvents, err := iface.ListEventsForAgent("other", 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgent: %v", err)
	}
	if len(agentEvents) != 1 {
		t.Errorf("expected 1 agent event, got %d", len(agentEvents))
	}

	// Locks
	lock, conflict, err := iface.AcquireLock("test.go", "test-agent", 1, 0, true, time.Hour)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if conflict != nil {
		t.Errorf("unexpected conflict: %+v", conflict)
	}
	if lock.Path != "test.go" {
		t.Errorf("expected lock path 'test.go', got %q", lock.Path)
	}

	locks, err := iface.ListLocks()
	if err != nil {
		t.Fatalf("ListLocks: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("expected 1 lock, got %d", len(locks))
	}

	agentLocks, err := iface.ListLocksForAgent("test-agent")
	if err != nil {
		t.Fatalf("ListLocksForAgent: %v", err)
	}
	if len(agentLocks) != 1 {
		t.Errorf("expected 1 agent lock, got %d", len(agentLocks))
	}

	if err := iface.ReleaseLock("test.go", "test-agent"); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	// Frontier
	ps, err := iface.GetActivePointstamps()
	if err != nil {
		t.Fatalf("GetActivePointstamps: %v", err)
	}
	if len(ps) != 1 {
		t.Errorf("expected 1 pointstamp, got %d", len(ps))
	}
}
