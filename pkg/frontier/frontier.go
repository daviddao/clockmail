// Package frontier computes Naiad-style progress tracking frontiers.
//
// The frontier is the antichain (set of mutually incomparable elements)
// of minimum active pointstamps across all agents. An agent can safely
// finalize output for timestamp t only when no other agent has outstanding
// work at any timestamp <= t.
//
// This enables fine-grained progress tracking without global barriers:
// agents working on independent epochs proceed freely, and the frontier
// tells each agent exactly when it is safe to commit.
package frontier

import "github.com/daviddao/clockmail/pkg/model"

// ComputeFrontier returns the antichain of minimal active pointstamps.
// A pointstamp p is in the frontier iff no other active pointstamp q
// satisfies q.Timestamp < p.Timestamp (strictly less).
func ComputeFrontier(active []model.Pointstamp) []model.Pointstamp {
	var frontier []model.Pointstamp
	for _, p := range active {
		dominated := false
		for _, q := range active {
			if q.AgentID != p.AgentID && q.Timestamp.Less(p.Timestamp) {
				dominated = true
				break
			}
		}
		if !dominated {
			frontier = append(frontier, p)
		}
	}
	return frontier
}

// FrontierStatus is the result of a frontier safety check for a specific
// agent at a specific timestamp.
type FrontierStatus struct {
	SafeToFinalize bool               `json:"safe_to_finalize"`
	Frontier       []model.Pointstamp `json:"frontier"`
	BlockedBy      []model.Pointstamp `json:"blocked_by,omitempty"`
}

// ComputeFrontierStatus checks whether agentID can safely finalize work
// at timestamp ts, given the set of active pointstamps from all agents.
//
// An agent is safe to finalize at ts when no other agent has outstanding
// work at any timestamp <= ts. The returned status includes the computed
// frontier and the list of blocking pointstamps (if any).
func ComputeFrontierStatus(agentID string, ts model.Timestamp, active []model.Pointstamp) FrontierStatus {
	f := ComputeFrontier(active)
	status := FrontierStatus{
		SafeToFinalize: true,
		Frontier:       f,
	}
	for _, p := range active {
		if p.AgentID == agentID {
			continue
		}
		if p.Timestamp.LessEq(ts) {
			status.SafeToFinalize = false
			status.BlockedBy = append(status.BlockedBy, p)
		}
	}
	return status
}
