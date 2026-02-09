package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Agent tests ---

func TestRegisterAgent(t *testing.T) {
	s := newTestStore(t)
	ag, err := s.RegisterAgent("alice")
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if ag.ID != "alice" {
		t.Fatalf("got ID %q, want alice", ag.ID)
	}
	if ag.Clock != 0 || ag.Epoch != 0 || ag.Round != 0 {
		t.Fatalf("new agent should have zero clock/epoch/round, got %d/%d/%d", ag.Clock, ag.Epoch, ag.Round)
	}
}

func TestRegisterAgent_Idempotent(t *testing.T) {
	s := newTestStore(t)
	a1, err := s.RegisterAgent("alice")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := s.RegisterAgent("alice")
	if err != nil {
		t.Fatal(err)
	}
	if a1.ID != a2.ID {
		t.Fatal("idempotent register should return same agent")
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestUpdateAgentClock(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	if err := s.UpdateAgentClock("alice", 42, 1, 2); err != nil {
		t.Fatalf("UpdateAgentClock: %v", err)
	}

	ag, err := s.GetAgent("alice")
	if err != nil {
		t.Fatal(err)
	}
	if ag.Clock != 42 || ag.Epoch != 1 || ag.Round != 2 {
		t.Fatalf("clock/epoch/round = %d/%d/%d, want 42/1/2", ag.Clock, ag.Epoch, ag.Round)
	}
}

func TestListAgents_Ordered(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("carol")
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 {
		t.Fatalf("got %d agents, want 3", len(agents))
	}
	if agents[0].ID != "alice" || agents[1].ID != "bob" || agents[2].ID != "carol" {
		t.Fatalf("agents not ordered: %v", []string{agents[0].ID, agents[1].ID, agents[2].ID})
	}
}

// --- Event tests ---

func TestInsertAndListEvents(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	e := &model.Event{
		AgentID:   "alice",
		LamportTS: 1,
		Kind:      model.EventMsg,
		Target:    "bob",
		Body:      "hello",
		CreatedAt: time.Now().UTC(),
	}
	id, err := s.InsertEvent(e)
	if err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}
	if id <= 0 {
		t.Fatalf("InsertEvent returned id %d, want > 0", id)
	}

	events, err := s.ListEvents(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Body != "hello" || events[0].Target != "bob" {
		t.Fatalf("event data mismatch: %+v", events[0])
	}
}

func TestListEvents_SinceTS(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	for i := int64(1); i <= 5; i++ {
		s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			Target: "bob", Body: "msg", CreatedAt: time.Now().UTC(),
		})
	}

	events, err := s.ListEvents(3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 { // ts 3, 4, 5
		t.Fatalf("got %d events since ts=3, want 3", len(events))
	}
}

func TestListEvents_Limit(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	for i := int64(1); i <= 10; i++ {
		s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			CreatedAt: time.Now().UTC(),
		})
	}

	events, err := s.ListEvents(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events with limit=3, want 3", len(events))
	}
}

func TestListEvents_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	// Insert 5 events, request with limit 0 (should default to 100)
	for i := int64(1); i <= 5; i++ {
		s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			CreatedAt: time.Now().UTC(),
		})
	}

	events, err := s.ListEvents(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("got %d events with default limit, want 5", len(events))
	}
}

func TestListEventsSinceID(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	var lastID int64
	for i := int64(1); i <= 5; i++ {
		id, _ := s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			CreatedAt: time.Now().UTC(),
		})
		if i == 2 {
			lastID = id
		}
	}

	events, err := s.ListEventsSinceID(lastID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 { // IDs 3, 4, 5
		t.Fatalf("got %d events since ID %d, want 3", len(events), lastID)
	}
}

func TestMaxEventID_Empty(t *testing.T) {
	s := newTestStore(t)
	if id := s.MaxEventID(); id != 0 {
		t.Fatalf("empty store: MaxEventID = %d, want 0", id)
	}
}

func TestMaxEventID_WithEvents(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	for i := int64(1); i <= 3; i++ {
		s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			CreatedAt: time.Now().UTC(),
		})
	}

	if id := s.MaxEventID(); id != 3 {
		t.Fatalf("MaxEventID = %d, want 3", id)
	}
}

func TestListEventsForAgent(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")

	// alice sends to bob
	s.InsertEvent(&model.Event{
		AgentID: "alice", LamportTS: 1, Kind: model.EventMsg,
		Target: "bob", Body: "hi bob", CreatedAt: time.Now().UTC(),
	})
	// alice sends to carol
	s.InsertEvent(&model.Event{
		AgentID: "alice", LamportTS: 2, Kind: model.EventMsg,
		Target: "carol", Body: "hi carol", CreatedAt: time.Now().UTC(),
	})

	events, err := s.ListEventsForAgent("bob", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events for bob, want 1", len(events))
	}
	if events[0].Body != "hi bob" {
		t.Fatalf("wrong event for bob: %q", events[0].Body)
	}
}

// --- Cursor tests ---

func TestCursor_DefaultZero(t *testing.T) {
	s := newTestStore(t)
	if c := s.GetCursor("alice"); c != 0 {
		t.Fatalf("default cursor = %d, want 0", c)
	}
}

func TestCursor_SetAndGet(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	if err := s.SetCursor("alice", 42); err != nil {
		t.Fatal(err)
	}
	if c := s.GetCursor("alice"); c != 42 {
		t.Fatalf("cursor = %d, want 42", c)
	}
}

func TestCursor_Update(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	s.SetCursor("alice", 10)
	s.SetCursor("alice", 20) // upsert
	if c := s.GetCursor("alice"); c != 20 {
		t.Fatalf("updated cursor = %d, want 20", c)
	}
}

// --- Lock tests ---

func TestAcquireLock_Success(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	lock, conflict, err := s.AcquireLock("file.go", "alice", 1, 0, true, time.Hour)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if conflict != nil {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
	if lock.Path != "file.go" || lock.AgentID != "alice" {
		t.Fatalf("lock mismatch: %+v", lock)
	}
}

func TestAcquireLock_Conflict(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")

	// alice acquires first with lower timestamp
	_, _, err := s.AcquireLock("file.go", "alice", 1, 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// bob tries with higher timestamp — should lose
	lock, conflict, err := s.AcquireLock("file.go", "bob", 2, 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if lock != nil {
		t.Fatal("bob should not have gotten the lock")
	}
	if conflict == nil {
		t.Fatal("expected conflict")
	}
	if conflict.AgentID != "alice" {
		t.Fatalf("conflict agent = %q, want alice", conflict.AgentID)
	}
}

func TestAcquireLock_Eviction(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")

	// bob acquires with higher timestamp
	_, _, err := s.AcquireLock("file.go", "bob", 10, 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// alice requests with lower timestamp — should win (evict bob)
	lock, conflict, err := s.AcquireLock("file.go", "alice", 1, 0, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Fatal("alice should have won via eviction")
	}
	if lock == nil || lock.AgentID != "alice" {
		t.Fatal("alice should hold the lock")
	}
}

func TestReleaseLock(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	s.AcquireLock("file.go", "alice", 1, 0, true, time.Hour)

	if err := s.ReleaseLock("file.go", "alice"); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	locks, err := s.ListLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 0 {
		t.Fatalf("after release: got %d locks, want 0", len(locks))
	}
}

func TestListLocks_ExpiresStale(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")

	// Acquire with 1ms TTL and wait for expiry
	s.AcquireLock("file.go", "alice", 1, 0, true, time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	locks, err := s.ListLocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 0 {
		t.Fatalf("stale lock should be expired, got %d locks", len(locks))
	}
}

func TestListLocksForAgent(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")

	s.AcquireLock("a.go", "alice", 1, 0, true, time.Hour)
	s.AcquireLock("b.go", "bob", 2, 0, true, time.Hour)

	locks, err := s.ListLocksForAgent("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 || locks[0].Path != "a.go" {
		t.Fatalf("alice locks: got %+v", locks)
	}
}

// --- Pointstamp / frontier integration ---

func TestGetActivePointstamps(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	s.RegisterAgent("bob")
	s.UpdateAgentClock("alice", 5, 1, 0)
	s.UpdateAgentClock("bob", 3, 0, 1)

	ps, err := s.GetActivePointstamps()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d pointstamps, want 2", len(ps))
	}
}

// --- Retry logic tests ---

func TestIsTransientSQLiteError_BusyError(t *testing.T) {
	err := fmt.Errorf("SQLITE_BUSY: database is locked")
	if !isTransientSQLiteError(err) {
		t.Fatal("SQLITE_BUSY should be transient")
	}
}

func TestIsTransientSQLiteError_LockedError(t *testing.T) {
	err := fmt.Errorf("SQLITE_LOCKED: database table is locked")
	if !isTransientSQLiteError(err) {
		t.Fatal("SQLITE_LOCKED should be transient")
	}
}

func TestIsTransientSQLiteError_IOError(t *testing.T) {
	err := fmt.Errorf("SQLITE_IOERR (522)")
	if !isTransientSQLiteError(err) {
		t.Fatal("SQLITE_IOERR should be transient")
	}
}

func TestIsTransientSQLiteError_NilError(t *testing.T) {
	if isTransientSQLiteError(nil) {
		t.Fatal("nil error should not be transient")
	}
}

func TestIsTransientSQLiteError_NonTransient(t *testing.T) {
	err := fmt.Errorf("UNIQUE constraint failed")
	if isTransientSQLiteError(err) {
		t.Fatal("UNIQUE constraint error should not be transient")
	}
}

func TestRetryOnContention_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := retryOnContention(func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryOnContention_SuccessAfterRetry(t *testing.T) {
	calls := 0
	err := retryOnContention(func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("SQLITE_BUSY: database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOnContention_NonTransientError(t *testing.T) {
	calls := 0
	err := retryOnContention(func() error {
		calls++
		return fmt.Errorf("UNIQUE constraint failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("non-transient error should not retry, got %d calls", calls)
	}
}

func TestRetryOnContention_ExhaustsRetries(t *testing.T) {
	calls := 0
	err := retryOnContention(func() error {
		calls++
		return fmt.Errorf("SQLITE_BUSY: database is locked")
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 4 { // 1 initial + 3 retries
		t.Fatalf("expected 4 calls (1 + 3 retries), got %d", calls)
	}
}

// --- CountEvents tests ---

func TestCountEvents_Empty(t *testing.T) {
	s := newTestStore(t)
	if c := s.CountEvents(); c != 0 {
		t.Fatalf("empty store: CountEvents = %d, want 0", c)
	}
}

func TestCountEvents_WithEvents(t *testing.T) {
	s := newTestStore(t)
	s.RegisterAgent("alice")
	for i := int64(1); i <= 5; i++ {
		s.InsertEvent(&model.Event{
			AgentID: "alice", LamportTS: i, Kind: model.EventMsg,
			CreatedAt: time.Now().UTC(),
		})
	}
	if c := s.CountEvents(); c != 5 {
		t.Fatalf("CountEvents = %d, want 5", c)
	}
}

// --- Helper tests ---

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Fatal("boolToInt(true) should be 1")
	}
	if boolToInt(false) != 0 {
		t.Fatal("boolToInt(false) should be 0")
	}
}
