package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daviddao/clockmail/internal/model"
)

func (a *app) cmdSend(args []string) int {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	agent := flags.String("agent", "", "sender agent ID")
	epoch := flags.Int64("epoch", -1, "epoch context (-1 = keep current)")
	round := flags.Int64("round", -1, "round context (-1 = keep current)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: cm send <to> <message> [--agent ID] [--json]")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	ep, rn := a.resolveEpochRound(agentID, *epoch, *round)
	to := flags.Arg(0)
	body := strings.Join(flags.Args()[1:], " ")

	c := a.getClock(agentID)
	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, ep, rn)

	recipients := strings.Split(to, ",")
	var eventIDs []int64
	for _, r := range recipients {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		id, err := a.store.InsertEvent(&model.Event{
			AgentID:   agentID,
			LamportTS: ts,
			Epoch:     ep,
			Round:     rn,
			Kind:      model.EventMsg,
			Target:    r,
			Body:      body,
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "cm: send: %v\n", err)
			return 1
		}
		eventIDs = append(eventIDs, id)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"lamport_ts": ts, "event_ids": eventIDs, "recipients": len(eventIDs),
		})
	} else {
		fmt.Printf("sent to %s at ts=%d (%d recipients)\n", to, ts, len(eventIDs))
	}
	return 0
}
