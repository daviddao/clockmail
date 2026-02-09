package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/daviddao/clockmail/pkg/frontier"
	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdSync(args []string) int {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID")
	epoch := flags.Int64("epoch", 0, "current working epoch")
	round := flags.Int64("round", 0, "current working round")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	// 1. Heartbeat: tick clock, update position.
	c := a.getClock(agentID)
	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, *epoch, *round)
	if _, err := a.store.InsertEvent(&model.Event{
		AgentID:   agentID,
		LamportTS: ts,
		Epoch:     *epoch,
		Round:     *round,
		Kind:      model.EventProgress,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "cm: sync: event: %v\n", err)
	}

	// 2. Recv: fetch new messages, apply IR2.
	since := a.store.GetCursor(agentID)
	messages, err := a.store.ListEventsForAgent(agentID, since, 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: sync: recv: %v\n", err)
		return 1
	}
	var maxMsgTS int64
	for _, e := range messages {
		c.Receive(e.LamportTS)
		if e.LamportTS > maxMsgTS {
			maxMsgTS = e.LamportTS
		}
	}
	newTS := c.Value()
	_ = a.store.UpdateAgentClock(agentID, newTS, *epoch, *round)
	if maxMsgTS > 0 {
		_ = a.store.SetCursor(agentID, maxMsgTS+1)
	}

	// 3. Frontier: check safety.
	nts := model.Timestamp{Epoch: *epoch, Round: *round}
	active, _ := a.store.GetActivePointstamps()
	fStatus := frontier.ComputeFrontierStatus(agentID, nts, active)

	// 4. Locks: show what this agent holds.
	locks, _ := a.store.ListLocksForAgent(agentID)

	if *jsonOut {
		printJSON(map[string]interface{}{
			"agent_id":         agentID,
			"lamport_ts":       newTS,
			"epoch":            *epoch,
			"round":            *round,
			"messages":         messages,
			"message_count":    len(messages),
			"frontier":         fStatus,
			"safe_to_finalize": fStatus.SafeToFinalize,
			"locks":            locks,
		})
	} else {
		// Messages FIRST — the most important output for agent coordination.
		if len(messages) > 0 {
			fmt.Printf("\n=== %d new message(s) ===\n", len(messages))
			for _, e := range messages {
				body := e.Body
				if len(body) > 120 {
					body = body[:120] + "..."
				}
				fmt.Printf("  [ts=%d] %s: %s\n", e.LamportTS, e.AgentID, body)
			}
			fmt.Println("========================")
			fmt.Println()
		}

		fmt.Printf("sync %s ts=%d epoch=%d round=%d\n", agentID, newTS, *epoch, *round)

		if fStatus.SafeToFinalize {
			fmt.Printf("  frontier: SAFE to finalize epoch=%d round=%d\n", *epoch, *round)
		} else {
			fmt.Printf("  frontier: NOT SAFE to finalize epoch=%d round=%d\n", *epoch, *round)
			for _, b := range fStatus.BlockedBy {
				fmt.Printf("    blocked by %s at epoch=%d round=%d\n",
					b.AgentID, b.Timestamp.Epoch, b.Timestamp.Round)
			}
		}

		if len(locks) > 0 {
			fmt.Printf("  %d active locks:\n", len(locks))
			for _, l := range locks {
				fmt.Printf("    %s (ts=%d, expires %s)\n",
					l.Path, l.LamportTS, l.ExpiresAt.Format(time.RFC3339))
			}
		}
	}
	return 0
}
