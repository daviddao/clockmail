package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
	"github.com/daviddao/clockmail/pkg/store"
)

// --- envOr tests ---

func TestEnvOr_EnvSet(t *testing.T) {
	t.Setenv("TEST_CM_ENV", "hello")
	if got := envOr("TEST_CM_ENV", "default"); got != "hello" {
		t.Fatalf("envOr with set env: got %q, want %q", got, "hello")
	}
}

func TestEnvOr_EnvUnset(t *testing.T) {
	if got := envOr("TEST_CM_UNSET_KEY_XYZ", "fallback"); got != "fallback" {
		t.Fatalf("envOr with unset env: got %q, want %q", got, "fallback")
	}
}

func TestEnvOr_EmptyEnv(t *testing.T) {
	t.Setenv("TEST_CM_EMPTY", "")
	if got := envOr("TEST_CM_EMPTY", "default"); got != "default" {
		t.Fatalf("envOr with empty env: got %q, want %q", got, "default")
	}
}

// --- agentTimestamp tests ---

func TestAgentTimestamp_Found(t *testing.T) {
	agents := []model.Agent{
		{ID: "alice", Epoch: 3, Round: 7},
		{ID: "bob", Epoch: 1, Round: 2},
	}
	ts := agentTimestamp(agents, "alice")
	if ts.Epoch != 3 || ts.Round != 7 {
		t.Fatalf("agentTimestamp(alice): got epoch=%d round=%d, want 3/7", ts.Epoch, ts.Round)
	}
}

func TestAgentTimestamp_NotFound(t *testing.T) {
	agents := []model.Agent{
		{ID: "alice", Epoch: 3, Round: 7},
	}
	ts := agentTimestamp(agents, "nonexistent")
	if ts.Epoch != 0 || ts.Round != 0 {
		t.Fatalf("agentTimestamp(nonexistent): got epoch=%d round=%d, want 0/0", ts.Epoch, ts.Round)
	}
}

func TestAgentTimestamp_EmptyList(t *testing.T) {
	ts := agentTimestamp(nil, "alice")
	if ts.Epoch != 0 || ts.Round != 0 {
		t.Fatal("agentTimestamp on nil list should return zero")
	}
}

// --- resolveAgent tests ---

func TestResolveAgent_FlagValue(t *testing.T) {
	a := &app{agentID: "env-agent"}
	got, err := a.resolveAgent("flag-agent")
	if err != nil || got != "flag-agent" {
		t.Fatalf("resolveAgent with flag: got %q, err=%v", got, err)
	}
}

func TestResolveAgent_EnvFallback(t *testing.T) {
	a := &app{agentID: "env-agent"}
	got, err := a.resolveAgent("")
	if err != nil || got != "env-agent" {
		t.Fatalf("resolveAgent with env: got %q, err=%v", got, err)
	}
}

func TestResolveAgent_NoAgent(t *testing.T) {
	a := &app{}
	_, err := a.resolveAgent("")
	if err == nil {
		t.Fatal("resolveAgent with no agent should return error")
	}
}

// --- resolveEpochRound tests ---

func newTestApp(t *testing.T) *app {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return &app{store: s, agentID: "test"}
}

func TestResolveEpochRound_ExplicitValues(t *testing.T) {
	a := newTestApp(t)
	ep, rn := a.resolveEpochRound("any", 5, 3)
	if ep != 5 || rn != 3 {
		t.Fatalf("explicit: got %d/%d, want 5/3", ep, rn)
	}
}

func TestResolveEpochRound_SentinelWithAgent(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.UpdateAgentClock("alice", 0, 2, 4)

	ep, rn := a.resolveEpochRound("alice", -1, -1)
	if ep != 2 || rn != 4 {
		t.Fatalf("sentinel with agent: got %d/%d, want 2/4", ep, rn)
	}
}

func TestResolveEpochRound_SentinelNoAgent(t *testing.T) {
	a := newTestApp(t)
	ep, rn := a.resolveEpochRound("nonexistent", -1, -1)
	if ep != 0 || rn != 0 {
		t.Fatalf("sentinel without agent: got %d/%d, want 0/0", ep, rn)
	}
}

func TestResolveEpochRound_MixedSentinel(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.UpdateAgentClock("alice", 0, 5, 9)

	ep, rn := a.resolveEpochRound("alice", 10, -1) // explicit epoch, sentinel round
	if ep != 10 || rn != 9 {
		t.Fatalf("mixed sentinel: got %d/%d, want 10/9", ep, rn)
	}
}

// --- printEvent tests ---

func TestPrintEvent_Msg(t *testing.T) {
	e := model.Event{
		LamportTS: 5, AgentID: "alice", Kind: model.EventMsg,
		Target: "bob", Body: "hello",
	}
	out := captureStdout(t, func() { printEvent(e) })
	if !strings.Contains(out, "alice -> bob: hello") {
		t.Fatalf("printEvent msg: got %q", out)
	}
	if !strings.Contains(out, "[ts=5]") {
		t.Fatalf("printEvent msg: missing timestamp in %q", out)
	}
}

func TestPrintEvent_LockReq(t *testing.T) {
	e := model.Event{
		LamportTS: 3, AgentID: "alice", Kind: model.EventLockReq,
		Target: "file.go",
	}
	out := captureStdout(t, func() { printEvent(e) })
	if !strings.Contains(out, "lock-req file.go") {
		t.Fatalf("printEvent lock_req: got %q", out)
	}
}

func TestPrintEvent_LockRel(t *testing.T) {
	e := model.Event{
		LamportTS: 4, AgentID: "alice", Kind: model.EventLockRel,
		Target: "file.go",
	}
	out := captureStdout(t, func() { printEvent(e) })
	if !strings.Contains(out, "unlock file.go") {
		t.Fatalf("printEvent lock_rel: got %q", out)
	}
}

func TestPrintEvent_Progress(t *testing.T) {
	e := model.Event{
		LamportTS: 6, AgentID: "alice", Kind: model.EventProgress,
		Epoch: 1, Round: 2,
	}
	out := captureStdout(t, func() { printEvent(e) })
	if !strings.Contains(out, "heartbeat epoch=1 round=2") {
		t.Fatalf("printEvent progress: got %q", out)
	}
}

// --- printInbox tests ---

func TestPrintInbox_Empty(t *testing.T) {
	count := printInbox(nil)
	if count != 0 {
		t.Fatalf("printInbox(nil) = %d, want 0", count)
	}
}

func TestPrintInbox_Count(t *testing.T) {
	msgs := []model.Event{
		{AgentID: "alice", LamportTS: 1, Body: "hi"},
		{AgentID: "bob", LamportTS: 2, Body: "hello"},
	}
	// Capture stderr since printInbox writes there
	out := captureStderr(t, func() { printInbox(msgs) })
	if !strings.Contains(out, "2 pending message(s)") {
		t.Fatalf("printInbox: expected count in output, got %q", out)
	}
}

func TestPrintInbox_Truncation(t *testing.T) {
	longBody := strings.Repeat("x", 200)
	msgs := []model.Event{
		{AgentID: "alice", LamportTS: 1, Body: longBody},
	}
	out := captureStderr(t, func() { printInbox(msgs) })
	if !strings.Contains(out, "...") {
		t.Fatal("printInbox should truncate long messages")
	}
	// The truncated body should be 120 chars + "..."
	if strings.Contains(out, strings.Repeat("x", 121)) {
		t.Fatal("printInbox truncation not applied at 120 chars")
	}
}

// --- injectAgentsSection tests ---

func TestInjectAgentsSection_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := injectAgentsSection(path); err != nil {
		t.Fatalf("injectAgentsSection new file: %v", err)
	}
	content, _ := os.ReadFile(path)
	text := string(content)
	if !strings.Contains(text, "# Agent Instructions") {
		t.Fatal("new file should contain header")
	}
	if !strings.Contains(text, agentsBeginMarker) {
		t.Fatal("new file should contain begin marker")
	}
	if !strings.Contains(text, agentsEndMarker) {
		t.Fatal("new file should contain end marker")
	}
}

func TestInjectAgentsSection_ExistingWithoutMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	os.WriteFile(path, []byte("# My Project\n\nExisting content.\n"), 0644)

	if err := injectAgentsSection(path); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	text := string(content)
	if !strings.Contains(text, "Existing content.") {
		t.Fatal("should preserve existing content")
	}
	if !strings.Contains(text, agentsBeginMarker) {
		t.Fatal("should append clockmail section")
	}
}

func TestInjectAgentsSection_ExistingWithMarkers_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	initial := "# Project\n\n" + agentsSection + "\nExtra stuff\n"
	os.WriteFile(path, []byte(initial), 0644)

	// Run inject twice — should be idempotent
	if err := injectAgentsSection(path); err != nil {
		t.Fatal(err)
	}
	if err := injectAgentsSection(path); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(path)
	text := string(content)

	// Should only have one pair of markers
	count := strings.Count(text, agentsBeginMarker)
	if count != 1 {
		t.Fatalf("expected 1 begin marker, got %d", count)
	}
	if !strings.Contains(text, "Extra stuff") {
		t.Fatal("should preserve content after markers")
	}
}

// --- drainInbox integration test ---

func TestDrainInbox_AdvancesClockAndCursor(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")

	// bob sends alice a message at ts=10
	a.store.InsertEvent(&model.Event{
		AgentID: "bob", LamportTS: 10, Kind: model.EventMsg,
		Target: "alice", Body: "hello", CreatedAt: time.Now().UTC(),
	})

	c := a.getClock("alice") // clock starts at 0
	msgs := a.drainInbox("alice", c)

	if len(msgs) != 1 {
		t.Fatalf("drainInbox: got %d msgs, want 1", len(msgs))
	}
	// Clock should advance to max(0, 10)+1 = 11
	if c.Value() < 11 {
		t.Fatalf("clock should advance past received ts, got %d", c.Value())
	}
	// Cursor should advance past the message
	cursor := a.store.GetCursor("alice")
	if cursor <= 10 {
		t.Fatalf("cursor should advance past ts=10, got %d", cursor)
	}
}

func TestDrainInbox_EmptyAgent(t *testing.T) {
	a := newTestApp(t)
	msgs := a.drainInbox("", nil)
	if msgs != nil {
		t.Fatal("drainInbox with empty agent should return nil")
	}
}

// --- peekInbox test ---

func TestPeekInbox_DoesNotAdvanceCursor(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")

	a.store.InsertEvent(&model.Event{
		AgentID: "bob", LamportTS: 5, Kind: model.EventMsg,
		Target: "alice", Body: "peek test", CreatedAt: time.Now().UTC(),
	})

	cursorBefore := a.store.GetCursor("alice")
	msgs, count := a.peekInbox("alice")
	cursorAfter := a.store.GetCursor("alice")

	if count != 1 || len(msgs) != 1 {
		t.Fatalf("peekInbox: got %d msgs, want 1", count)
	}
	if cursorBefore != cursorAfter {
		t.Fatal("peekInbox should not advance cursor")
	}
}

// --- filterByFrom tests ---

func TestFilterByFrom_MatchingSender(t *testing.T) {
	events := []model.Event{
		{AgentID: "alice", LamportTS: 1, Body: "hi"},
		{AgentID: "bob", LamportTS: 2, Body: "hello"},
		{AgentID: "alice", LamportTS: 3, Body: "again"},
	}
	filtered := filterByFrom(events, "alice")
	if len(filtered) != 2 {
		t.Fatalf("filterByFrom(alice): got %d, want 2", len(filtered))
	}
	if filtered[0].Body != "hi" || filtered[1].Body != "again" {
		t.Fatal("filterByFrom should preserve order and correct messages")
	}
}

func TestFilterByFrom_NoMatch(t *testing.T) {
	events := []model.Event{
		{AgentID: "alice", LamportTS: 1, Body: "hi"},
	}
	filtered := filterByFrom(events, "bob")
	if len(filtered) != 0 {
		t.Fatalf("filterByFrom(bob): got %d, want 0", len(filtered))
	}
}

func TestFilterByFrom_EmptyEvents(t *testing.T) {
	filtered := filterByFrom(nil, "alice")
	if len(filtered) != 0 {
		t.Fatal("filterByFrom on nil should return empty")
	}
}

func TestFilterByFrom_PreservesLamportOrder(t *testing.T) {
	// Verify that filtering preserves Lamport timestamp ordering.
	// Per Lamport 1978, the order of events is meaningful — filtering
	// should not reorder them.
	events := []model.Event{
		{AgentID: "alice", LamportTS: 1, Body: "first"},
		{AgentID: "bob", LamportTS: 2, Body: "skip"},
		{AgentID: "alice", LamportTS: 5, Body: "second"},
		{AgentID: "charlie", LamportTS: 6, Body: "skip"},
		{AgentID: "alice", LamportTS: 10, Body: "third"},
	}
	filtered := filterByFrom(events, "alice")
	if len(filtered) != 3 {
		t.Fatalf("got %d, want 3", len(filtered))
	}
	for i := 1; i < len(filtered); i++ {
		if filtered[i].LamportTS <= filtered[i-1].LamportTS {
			t.Fatalf("Lamport order violated: ts[%d]=%d <= ts[%d]=%d",
				i, filtered[i].LamportTS, i-1, filtered[i-1].LamportTS)
		}
	}
}

// --- resolveRecipients tests ---

func TestResolveRecipients_CommaSeparated(t *testing.T) {
	a := newTestApp(t)
	r, err := a.resolveRecipients("alice,bob,charlie", "sender")
	if err != nil {
		t.Fatalf("resolveRecipients: %v", err)
	}
	if len(r) != 3 || r[0] != "alice" || r[1] != "bob" || r[2] != "charlie" {
		t.Fatalf("resolveRecipients: got %v", r)
	}
}

func TestResolveRecipients_SingleRecipient(t *testing.T) {
	a := newTestApp(t)
	r, err := a.resolveRecipients("alice", "sender")
	if err != nil {
		t.Fatalf("resolveRecipients: %v", err)
	}
	if len(r) != 1 || r[0] != "alice" {
		t.Fatalf("resolveRecipients: got %v", r)
	}
}

func TestResolveRecipients_BroadcastAll(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("sender")
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")

	r, err := a.resolveRecipients("all", "sender")
	if err != nil {
		t.Fatalf("resolveRecipients all: %v", err)
	}
	if len(r) != 2 {
		t.Fatalf("resolveRecipients all: got %d recipients, want 2 (excluding sender)", len(r))
	}
	// Should contain alice and bob but not sender
	found := map[string]bool{}
	for _, id := range r {
		found[id] = true
	}
	if found["sender"] {
		t.Fatal("broadcast should exclude sender")
	}
	if !found["alice"] || !found["bob"] {
		t.Fatalf("broadcast should include alice and bob, got %v", r)
	}
}

func TestResolveRecipients_BroadcastAllCaseInsensitive(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("sender")
	a.store.RegisterAgent("alice")

	r, err := a.resolveRecipients("ALL", "sender")
	if err != nil {
		t.Fatalf("resolveRecipients ALL: %v", err)
	}
	if len(r) != 1 || r[0] != "alice" {
		t.Fatalf("resolveRecipients ALL: got %v", r)
	}
}

func TestResolveRecipients_BroadcastNoOtherAgents(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("lonely")

	_, err := a.resolveRecipients("all", "lonely")
	if err == nil {
		t.Fatal("broadcast with no other agents should return error")
	}
}

func TestResolveRecipients_EmptyString(t *testing.T) {
	a := newTestApp(t)
	_, err := a.resolveRecipients("", "sender")
	if err == nil {
		t.Fatal("empty recipients should return error")
	}
}

func TestResolveRecipients_TrimsWhitespace(t *testing.T) {
	a := newTestApp(t)
	r, err := a.resolveRecipients(" alice , bob ", "sender")
	if err != nil {
		t.Fatalf("resolveRecipients: %v", err)
	}
	if len(r) != 2 || r[0] != "alice" || r[1] != "bob" {
		t.Fatalf("resolveRecipients: got %v", r)
	}
}

// --- agentPresence tests ---

func TestAgentPresence_Online(t *testing.T) {
	ag := model.Agent{ID: "alice", LastSeen: time.Now().Add(-30 * time.Second)}
	if got := agentPresence(ag); got != "online" {
		t.Fatalf("agentPresence 30s ago: got %q, want online", got)
	}
}

func TestAgentPresence_Idle(t *testing.T) {
	ag := model.Agent{ID: "alice", LastSeen: time.Now().Add(-5 * time.Minute)}
	if got := agentPresence(ag); got != "idle" {
		t.Fatalf("agentPresence 5min ago: got %q, want idle", got)
	}
}

func TestAgentPresence_Offline(t *testing.T) {
	ag := model.Agent{ID: "alice", LastSeen: time.Now().Add(-15 * time.Minute)}
	if got := agentPresence(ag); got != "offline" {
		t.Fatalf("agentPresence 15min ago: got %q, want offline", got)
	}
}

func TestAgentPresence_Boundary_Online(t *testing.T) {
	// Exactly at 2 minute boundary should be idle (>= 2min)
	ag := model.Agent{ID: "alice", LastSeen: time.Now().Add(-2*time.Minute - time.Second)}
	if got := agentPresence(ag); got != "idle" {
		t.Fatalf("agentPresence at 2min+1s: got %q, want idle", got)
	}
}

func TestAgentPresence_Boundary_Idle(t *testing.T) {
	// Exactly at 10 minute boundary should be offline (>= 10min)
	ag := model.Agent{ID: "alice", LastSeen: time.Now().Add(-10*time.Minute - time.Second)}
	if got := agentPresence(ag); got != "offline" {
		t.Fatalf("agentPresence at 10min+1s: got %q, want offline", got)
	}
}

// --- presenceIndicator tests ---

func TestPresenceIndicator_Online(t *testing.T) {
	if got := presenceIndicator("online"); got != "[+]" {
		t.Fatalf("presenceIndicator(online): got %q, want [+]", got)
	}
}

func TestPresenceIndicator_Idle(t *testing.T) {
	if got := presenceIndicator("idle"); got != "[~]" {
		t.Fatalf("presenceIndicator(idle): got %q, want [~]", got)
	}
}

func TestPresenceIndicator_Offline(t *testing.T) {
	if got := presenceIndicator("offline"); got != "[-]" {
		t.Fatalf("presenceIndicator(offline): got %q, want [-]", got)
	}
}

func TestPresenceIndicator_Unknown(t *testing.T) {
	if got := presenceIndicator("unknown"); got != "[-]" {
		t.Fatalf("presenceIndicator(unknown): got %q, want [-]", got)
	}
}

// --- gate command tests ---

func TestGateCheck_SafeWhenAllAgentsPastEpoch(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	// Both agents at epoch 2, checking if epoch 1 is safe
	a.store.UpdateAgentClock("alice", 10, 2, 0)
	a.store.UpdateAgentClock("bob", 8, 2, 0)

	out := captureStdout(t, func() {
		code := a.gateCheck("alice", model.Timestamp{Epoch: 1, Round: 0}, false)
		if code != 0 {
			t.Fatalf("gateCheck: expected exit 0 (safe), got %d", code)
		}
	})
	if !strings.Contains(out, "SAFE") {
		t.Fatalf("gateCheck output should contain SAFE, got %q", out)
	}
}

func TestGateCheck_NotSafeWhenAgentBehind(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	// alice at epoch 2, bob still at epoch 1 — epoch 1 not safe
	a.store.UpdateAgentClock("alice", 10, 2, 0)
	a.store.UpdateAgentClock("bob", 8, 1, 0)

	out := captureStdout(t, func() {
		code := a.gateCheck("alice", model.Timestamp{Epoch: 1, Round: 0}, false)
		if code != 2 {
			t.Fatalf("gateCheck: expected exit 2 (not safe), got %d", code)
		}
	})
	if !strings.Contains(out, "NOT SAFE") {
		t.Fatalf("gateCheck output should contain NOT SAFE, got %q", out)
	}
	if !strings.Contains(out, "bob") {
		t.Fatalf("gateCheck output should show blocker 'bob', got %q", out)
	}
}

func TestGateCheck_JSON(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 2, 0)
	a.store.UpdateAgentClock("bob", 8, 2, 0)

	out := captureStdout(t, func() {
		code := a.gateCheck("alice", model.Timestamp{Epoch: 1, Round: 0}, true)
		if code != 0 {
			t.Fatalf("gateCheck JSON: expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(out, `"safe"`) {
		t.Fatalf("gateCheck JSON output should contain 'safe' field, got %q", out)
	}
	if !strings.Contains(out, `"mode"`) {
		t.Fatalf("gateCheck JSON output should contain 'mode' field, got %q", out)
	}
}

func TestGateCheck_NotSafe_JSON(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 2, 0)
	a.store.UpdateAgentClock("bob", 8, 0, 0) // bob behind

	out := captureStdout(t, func() {
		code := a.gateCheck("alice", model.Timestamp{Epoch: 1, Round: 0}, true)
		if code != 2 {
			t.Fatalf("gateCheck JSON not safe: expected exit 2, got %d", code)
		}
	})
	if !strings.Contains(out, `"safe"`) {
		t.Fatalf("gateCheck JSON not safe output should contain 'safe' field, got %q", out)
	}
}

func TestCheckFrontierSafe_Safe(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 3, 0)
	a.store.UpdateAgentClock("bob", 8, 3, 0)

	if !a.checkFrontierSafe("alice", model.Timestamp{Epoch: 2, Round: 0}) {
		t.Fatal("checkFrontierSafe: should be safe when all agents past epoch")
	}
}

func TestCheckFrontierSafe_NotSafe(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 3, 0)
	a.store.UpdateAgentClock("bob", 8, 1, 0)

	if a.checkFrontierSafe("alice", model.Timestamp{Epoch: 2, Round: 0}) {
		t.Fatal("checkFrontierSafe: should NOT be safe when bob at epoch 1")
	}
}

func TestGateWait_ImmediatelySafe(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 5, 0)
	a.store.UpdateAgentClock("bob", 8, 5, 0)

	out := captureStdout(t, func() {
		code := a.gateWait("alice", model.Timestamp{Epoch: 2, Round: 0},
			5*time.Second, 100*time.Millisecond, false)
		if code != 0 {
			t.Fatalf("gateWait immediately safe: expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(out, "SAFE") {
		t.Fatalf("gateWait output should contain SAFE, got %q", out)
	}
}

func TestGateWait_ImmediatelySafe_JSON(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 5, 0)
	a.store.UpdateAgentClock("bob", 8, 5, 0)

	out := captureStdout(t, func() {
		code := a.gateWait("alice", model.Timestamp{Epoch: 2, Round: 0},
			5*time.Second, 100*time.Millisecond, true)
		if code != 0 {
			t.Fatalf("gateWait immediately safe JSON: expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(out, `"safe"`) {
		t.Fatalf("gateWait JSON output should contain 'safe' field, got %q", out)
	}
	if !strings.Contains(out, `"wait"`) {
		t.Fatalf("gateWait JSON output should contain 'wait' mode, got %q", out)
	}
}

func TestGateWait_Timeout(t *testing.T) {
	a := newTestApp(t)
	a.store.RegisterAgent("alice")
	a.store.RegisterAgent("bob")
	a.store.UpdateAgentClock("alice", 10, 5, 0)
	a.store.UpdateAgentClock("bob", 8, 0, 0) // bob far behind

	// Use a very short timeout so test completes quickly
	stderr := captureStderr(t, func() {
		code := a.gateWait("alice", model.Timestamp{Epoch: 4, Round: 0},
			200*time.Millisecond, 50*time.Millisecond, false)
		if code != 1 {
			t.Fatalf("gateWait timeout: expected exit 1, got %d", code)
		}
	})
	if !strings.Contains(stderr, "TIMEOUT") {
		t.Fatalf("gateWait timeout output should contain TIMEOUT, got %q", stderr)
	}
}

// --- Helpers ---

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
