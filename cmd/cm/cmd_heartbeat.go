package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/daviddao/clockmail/internal/model"
)

func (a *app) cmdHeartbeat(args []string) int {
	flags := flag.NewFlagSet("heartbeat", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID (overrides CLOCKMAIL_AGENT)")
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

	c := a.getClock(agentID)
	ts := c.Tick()

	if err := a.store.UpdateAgentClock(agentID, ts, *epoch, *round); err != nil {
		fmt.Fprintf(os.Stderr, "cm: heartbeat: %v\n", err)
		return 1
	}

	if _, err := a.store.InsertEvent(&model.Event{
		AgentID:   agentID,
		LamportTS: ts,
		Epoch:     *epoch,
		Round:     *round,
		Kind:      model.EventProgress,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "cm: heartbeat: event: %v\n", err)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"agent_id": agentID, "lamport_ts": ts, "epoch": *epoch, "round": *round,
		})
	} else {
		fmt.Printf("heartbeat %s ts=%d epoch=%d round=%d\n", agentID, ts, *epoch, *round)
	}
	return 0
}
