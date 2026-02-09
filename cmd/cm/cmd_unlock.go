package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdUnlock(args []string) int {
	flags := flag.NewFlagSet("unlock", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent releasing the lock")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: cm unlock <path> [--agent ID] [--json]")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	path := flags.Arg(0)
	c := a.getClock(agentID)
	ts := c.Tick()

	if _, err := a.store.InsertEvent(&model.Event{
		AgentID:   agentID,
		LamportTS: ts,
		Kind:      model.EventLockRel,
		Target:    path,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "cm: unlock: event: %v\n", err)
	}

	if err := a.store.ReleaseLock(path, agentID); err != nil {
		fmt.Fprintf(os.Stderr, "cm: unlock: %v\n", err)
		return 1
	}

	if *jsonOut {
		printJSON(map[string]interface{}{"released": true, "path": path, "lamport_ts": ts})
	} else {
		fmt.Printf("unlocked %s (ts=%d)\n", path, ts)
	}
	return 0
}
