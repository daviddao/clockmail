package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/daviddao/clockmail/pkg/frontier"
	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdPrime(args []string) int {
	flags := flag.NewFlagSet("prime", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, _ := a.resolveAgent(*agent)

	agents, _ := a.store.ListAgents()
	locks, _ := a.store.ListLocks()
	active, _ := a.store.GetActivePointstamps()
	f := frontier.ComputeFrontier(active)

	// Find current agent.
	var myAgent *model.Agent
	for i := range agents {
		if agents[i].ID == agentID {
			myAgent = &agents[i]
			break
		}
	}

	// Pending messages.
	var pendingMsgs []model.Event
	if agentID != "" {
		cursor := a.store.GetCursor(agentID)
		pendingMsgs, _ = a.store.ListEventsForAgent(agentID, cursor, 1000)
	}

	// My locks.
	var myLocks []model.Lock
	var otherLocks []model.Lock
	for _, l := range locks {
		if l.AgentID == agentID {
			myLocks = append(myLocks, l)
		} else {
			otherLocks = append(otherLocks, l)
		}
	}

	// Frontier safety.
	var fStatus *frontier.FrontierStatus
	if myAgent != nil {
		ts := model.Timestamp{Epoch: myAgent.Epoch, Round: myAgent.Round}
		s := frontier.ComputeFrontierStatus(agentID, ts, active)
		fStatus = &s
	}

	if *jsonOut {
		result := map[string]interface{}{
			"agent_id":         agentID,
			"agent":            myAgent,
			"agents":           agents,
			"locks":            locks,
			"my_locks":         myLocks,
			"other_locks":      otherLocks,
			"frontier":         f,
			"frontier_status":  fStatus,
			"pending_messages": pendingMsgs,
			"pending_count":    len(pendingMsgs),
		}
		printJSON(result)
		return 0
	}

	// --- Text Output ---

	fmt.Println("# Clockmail Coordination Context")
	fmt.Println()

	if myAgent != nil {
		fmt.Printf("Agent: %s | Clock: %d | Epoch: %d | Round: %d\n",
			myAgent.ID, myAgent.Clock, myAgent.Epoch, myAgent.Round)
	} else if agentID != "" {
		fmt.Printf("Agent: %s (not registered — run: cm register %s)\n", agentID, agentID)
	} else {
		fmt.Println("Agent: (not set — export CLOCKMAIL_AGENT=<id> && cm register <id>)")
	}
	fmt.Println()

	if len(agents) > 0 {
		fmt.Println("## Active Agents")
		for _, ag := range agents {
			stale := ""
			if time.Since(ag.LastSeen) > 10*time.Minute {
				stale = " (stale)"
			}
			marker := ""
			if ag.ID == agentID {
				marker = " (you)"
			}
			fmt.Printf("  %-15s clock=%-4d epoch=%-3d round=%-3d%s%s\n",
				ag.ID, ag.Clock, ag.Epoch, ag.Round, stale, marker)
		}
		fmt.Println()
	}

	if len(myLocks) > 0 {
		fmt.Println("## Your Locks")
		for _, l := range myLocks {
			remaining := time.Until(l.ExpiresAt).Truncate(time.Minute)
			fmt.Printf("  %s (expires in %s)\n", l.Path, remaining)
		}
		fmt.Println()
	}

	if len(otherLocks) > 0 {
		fmt.Println("## Other Agents' Locks")
		for _, l := range otherLocks {
			fmt.Printf("  %s held by %s\n", l.Path, l.AgentID)
		}
		fmt.Println()
	}

	if len(pendingMsgs) > 0 {
		fmt.Printf("## Pending Messages: %d\n", len(pendingMsgs))
		for _, e := range pendingMsgs {
			body := e.Body
			if len(body) > 100 {
				body = body[:100] + "..."
			}
			fmt.Printf("  [ts=%d] %s: %s\n", e.LamportTS, e.AgentID, body)
		}
		fmt.Println("  Run: cm recv   (to acknowledge and advance cursor)")
	} else {
		fmt.Println("## Pending Messages: 0")
	}
	fmt.Println()

	if myAgent != nil && fStatus != nil {
		fmt.Println("## Frontier")
		if fStatus.SafeToFinalize {
			fmt.Printf("  SAFE to finalize epoch=%d round=%d\n", myAgent.Epoch, myAgent.Round)
		} else {
			fmt.Printf("  NOT SAFE to finalize epoch=%d round=%d\n", myAgent.Epoch, myAgent.Round)
			for _, b := range fStatus.BlockedBy {
				fmt.Printf("    blocked by %s at epoch=%d round=%d\n",
					b.AgentID, b.Timestamp.Epoch, b.Timestamp.Round)
			}
		}
		if len(f) > 0 {
			fmt.Println("  Frontier points:")
			for _, p := range f {
				fmt.Printf("    %s @ epoch=%d round=%d\n",
					p.AgentID, p.Timestamp.Epoch, p.Timestamp.Round)
			}
		}
		fmt.Println()
	}

	fmt.Println("## Session Close Protocol")
	fmt.Println()
	fmt.Println("Before ending your session:")
	if len(myLocks) > 0 {
		fmt.Println("  1. Release all locks:")
		for _, l := range myLocks {
			fmt.Printf("     cm unlock %s\n", l.Path)
		}
		fmt.Println("  2. Sync your state:")
		fmt.Println("     cm sync --epoch <N>")
	} else {
		fmt.Println("  cm sync --epoch <N>")
	}
	fmt.Println()

	fmt.Println("## Quick Reference")
	fmt.Println()
	fmt.Println("  cm sync --epoch N     # Main loop: heartbeat + recv + frontier")
	fmt.Println("  cm send <to> <msg>    # Message another agent (drains inbox first)")
	fmt.Println("  cm send all <msg>     # Broadcast to all agents")
	fmt.Println("  cm broadcast <msg>    # Same as send all")
	fmt.Println("  cm lock <path>        # Lock file before editing")
	fmt.Println("  cm unlock <path>      # Release lock")
	fmt.Println("  cm status             # Full overview")
	fmt.Println("  cm log                # Event history")

	return 0
}
