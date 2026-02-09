package frontier

import (
	"testing"

	"github.com/daviddao/clockmail/pkg/model"
)

func ts(e, r int64) model.Timestamp {
	return model.Timestamp{Epoch: e, Round: r}
}

func ps(agent string, e, r int64) model.Pointstamp {
	return model.Pointstamp{AgentID: agent, Timestamp: ts(e, r)}
}

func TestComputeFrontier_Empty(t *testing.T) {
	f := ComputeFrontier(nil)
	if len(f) != 0 {
		t.Fatalf("empty input: got %d frontier points, want 0", len(f))
	}
}

func TestComputeFrontier_SingleAgent(t *testing.T) {
	active := []model.Pointstamp{ps("alice", 1, 0)}
	f := ComputeFrontier(active)
	if len(f) != 1 {
		t.Fatalf("single agent: got %d frontier points, want 1", len(f))
	}
	if f[0].AgentID != "alice" {
		t.Fatalf("single agent: got %q, want alice", f[0].AgentID)
	}
}

func TestComputeFrontier_TwoAgentsSameTimestamp(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
		ps("bob", 1, 0),
	}
	f := ComputeFrontier(active)
	if len(f) != 2 {
		t.Fatalf("same timestamp: got %d frontier points, want 2", len(f))
	}
}

func TestComputeFrontier_OneDominates(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
		ps("bob", 2, 1), // dominated by alice (1,0) < (2,1)
	}
	f := ComputeFrontier(active)
	if len(f) != 1 {
		t.Fatalf("one dominates: got %d frontier points, want 1", len(f))
	}
	if f[0].AgentID != "alice" {
		t.Fatalf("one dominates: frontier agent = %q, want alice", f[0].AgentID)
	}
}

func TestComputeFrontier_Incomparable(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 2), // epoch < bob, round > bob — incomparable
		ps("bob", 2, 1),
	}
	f := ComputeFrontier(active)
	if len(f) != 2 {
		t.Fatalf("incomparable: got %d frontier points, want 2", len(f))
	}
}

func TestComputeFrontier_ThreeAgents_MixedDomination(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 0, 0), // minimal
		ps("bob", 1, 1),   // dominated by alice
		ps("carol", 2, 0), // NOT dominated: (0,0) < (2,0) but alice has (0,0)
	}
	f := ComputeFrontier(active)
	// alice(0,0) is minimal; bob(1,1) dominated by alice; carol(2,0) dominated by alice (0,0) < (2,0)
	if len(f) != 1 {
		t.Fatalf("three agents: got %d frontier points, want 1 (alice)", len(f))
	}
	if f[0].AgentID != "alice" {
		t.Fatalf("three agents: got %q, want alice", f[0].AgentID)
	}
}

func TestComputeFrontierStatus_Safe(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
		ps("bob", 2, 0),
	}
	// alice at (1,0): bob is at (2,0), which is NOT <= (1,0), so alice is safe
	status := ComputeFrontierStatus("alice", ts(1, 0), active)
	if !status.SafeToFinalize {
		t.Fatal("alice should be safe to finalize")
	}
	if len(status.BlockedBy) != 0 {
		t.Fatalf("alice blocked by %d agents, want 0", len(status.BlockedBy))
	}
}

func TestComputeFrontierStatus_Blocked(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
		ps("bob", 1, 0),
	}
	// alice at (1,0): bob is at (1,0) which IS <= (1,0), so alice is blocked
	status := ComputeFrontierStatus("alice", ts(1, 0), active)
	if status.SafeToFinalize {
		t.Fatal("alice should be blocked")
	}
	if len(status.BlockedBy) != 1 || status.BlockedBy[0].AgentID != "bob" {
		t.Fatalf("alice should be blocked by bob, got %v", status.BlockedBy)
	}
}

func TestComputeFrontierStatus_BlockedByLowerTimestamp(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 2, 0),
		ps("bob", 1, 0), // bob at (1,0) <= alice's (2,0)
	}
	status := ComputeFrontierStatus("alice", ts(2, 0), active)
	if status.SafeToFinalize {
		t.Fatal("alice should be blocked by bob at lower timestamp")
	}
}

func TestComputeFrontierStatus_IgnoresSelf(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
	}
	// Only alice in the system — should be safe (ignores own pointstamp)
	status := ComputeFrontierStatus("alice", ts(1, 0), active)
	if !status.SafeToFinalize {
		t.Fatal("sole agent should be safe")
	}
}

func TestComputeFrontierStatus_EmptyActive(t *testing.T) {
	status := ComputeFrontierStatus("alice", ts(0, 0), nil)
	if !status.SafeToFinalize {
		t.Fatal("empty active set should be safe")
	}
}

func TestComputeFrontierStatus_IncludesFrontier(t *testing.T) {
	active := []model.Pointstamp{
		ps("alice", 1, 0),
		ps("bob", 2, 0),
	}
	status := ComputeFrontierStatus("alice", ts(1, 0), active)
	if len(status.Frontier) == 0 {
		t.Fatal("status should include computed frontier")
	}
}
