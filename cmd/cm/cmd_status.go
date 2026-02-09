package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/daviddao/clockmail/pkg/frontier"
	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdStatus(args []string) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID (optional, shows focused view)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Best-effort agent resolution (status works without one).
	agentID, _ := a.resolveAgent(*agent)

	agents, err := a.store.ListAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: status: %v\n", err)
		return 1
	}

	locks, _ := a.store.ListLocks()
	active, _ := a.store.GetActivePointstamps()
	f := frontier.ComputeFrontier(active)

	// Compute presence for each agent.
	type agentInfo struct {
		model.Agent
		Presence string `json:"presence"`
	}
	agentInfos := make([]agentInfo, len(agents))
	for i, ag := range agents {
		agentInfos[i] = agentInfo{Agent: ag, Presence: agentPresence(ag)}
	}

	if *jsonOut {
		result := map[string]interface{}{
			"agents":   agentInfos,
			"locks":    locks,
			"frontier": f,
		}
		if agentID != "" {
			ts := agentTimestamp(agents, agentID)
			result["my_status"] = frontier.ComputeFrontierStatus(agentID, ts, active)
		}
		printJSON(result)
	} else {
		fmt.Println("agents:")
		for _, ai := range agentInfos {
			marker := ""
			if ai.ID == agentID {
				marker = " <-- you"
			}
			presence := presenceIndicator(ai.Presence)
			fmt.Printf("  %s %-20s clock=%-4d epoch=%-3d round=%-3d last_seen=%s%s\n",
				presence, ai.ID, ai.Clock, ai.Epoch, ai.Round,
				ai.LastSeen.Format("15:04:05"), marker)
		}

		if len(locks) > 0 {
			fmt.Println("locks:")
			for _, l := range locks {
				fmt.Printf("  %-30s held by %-15s ts=%-4d expires=%s\n",
					l.Path, l.AgentID, l.LamportTS, l.ExpiresAt.Format("15:04:05"))
			}
		} else {
			fmt.Println("locks: none")
		}

		if len(f) > 0 {
			fmt.Println("frontier:")
			for _, p := range f {
				fmt.Printf("  %s @ epoch=%d round=%d\n",
					p.AgentID, p.Timestamp.Epoch, p.Timestamp.Round)
			}
		}

		if agentID != "" {
			ts := agentTimestamp(agents, agentID)
			fStatus := frontier.ComputeFrontierStatus(agentID, ts, active)
			if fStatus.SafeToFinalize {
				fmt.Printf("you (%s): SAFE to finalize epoch=%d round=%d\n",
					agentID, ts.Epoch, ts.Round)
			} else {
				fmt.Printf("you (%s): NOT SAFE to finalize epoch=%d round=%d\n",
					agentID, ts.Epoch, ts.Round)
			}
		}
	}
	return 0
}

func agentTimestamp(agents []model.Agent, id string) model.Timestamp {
	for _, ag := range agents {
		if ag.ID == id {
			return model.Timestamp{Epoch: ag.Epoch, Round: ag.Round}
		}
	}
	return model.Timestamp{}
}

// agentPresence returns a presence string based on last_seen time.
//   - "online"  — seen within 2 minutes
//   - "idle"    — seen within 10 minutes
//   - "offline" — not seen for 10+ minutes
func agentPresence(ag model.Agent) string {
	since := time.Since(ag.LastSeen)
	switch {
	case since < 2*time.Minute:
		return "online"
	case since < 10*time.Minute:
		return "idle"
	default:
		return "offline"
	}
}

// presenceIndicator returns a short text indicator for display.
func presenceIndicator(presence string) string {
	switch presence {
	case "online":
		return "[+]"
	case "idle":
		return "[~]"
	default:
		return "[-]"
	}
}
