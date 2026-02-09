package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdLock(args []string) int {
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	agent := flags.String("agent", "", "requesting agent ID")
	ttlSec := flags.Int("ttl", 3600, "lock TTL in seconds")
	epoch := flags.Int64("epoch", -1, "epoch context (-1 = keep current)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: cm lock <path> [--agent ID] [--ttl N] [--json]")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	path := flags.Arg(0)
	ep, rn := a.resolveEpochRound(agentID, *epoch, -1)

	c := a.getClock(agentID)

	// Auto-recv: show pending messages (lock holders may have sent releases).
	inbox := a.drainInbox(agentID, c)
	if !*jsonOut {
		printInbox(inbox)
	}

	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, ep, rn)

	ttl := time.Duration(*ttlSec) * time.Second
	lock, conflict, err := a.store.AcquireLock(path, agentID, ts, ep, true, ttl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: lock: %v\n", err)
		return 1
	}

	// Log the lock request event after the decision (avoids logging phantom requests).
	if _, err := a.store.InsertEvent(&model.Event{
		AgentID:   agentID,
		LamportTS: ts,
		Epoch:     ep,
		Kind:      model.EventLockReq,
		Target:    path,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "cm: lock: event: %v\n", err)
	}

	if conflict != nil {
		if *jsonOut {
			printJSON(map[string]interface{}{
				"granted":  false,
				"conflict": conflict,
				"resolution": fmt.Sprintf("%s holds lock with lower total order (%d,%q) vs (%d,%q)",
					conflict.AgentID, conflict.LamportTS, conflict.AgentID, ts, agentID),
				"inbox": inbox, "inbox_count": len(inbox),
			})
		} else {
			fmt.Printf("DENIED: %s holds %s (ts=%d < %d)\n",
				conflict.AgentID, path, conflict.LamportTS, ts)
		}
		return 2
	}

	if *jsonOut {
		printJSON(map[string]interface{}{"granted": true, "lock": lock, "lamport_ts": ts,
			"inbox": inbox, "inbox_count": len(inbox)})
	} else {
		fmt.Printf("locked %s (ts=%d, ttl=%ds)\n", path, ts, *ttlSec)
	}
	return 0
}
