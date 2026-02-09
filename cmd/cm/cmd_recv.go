package main

import (
	"flag"
	"fmt"
	"os"
)

func (a *app) cmdRecv(args []string) int {
	flags := flag.NewFlagSet("recv", flag.ContinueOnError)
	agent := flags.String("agent", "", "recipient agent ID")
	sinceTS := flags.Int64("since", -1, "fetch events with lamport_ts >= this (-1 = use cursor)")
	limit := flags.Int("limit", 100, "max messages to return")
	summary := flags.Bool("summary", false, "show one-line summaries only (first 80 chars)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	since := *sinceTS
	if since < 0 {
		since = a.store.GetCursor(agentID)
	}

	events, err := a.store.ListEventsForAgent(agentID, since, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: recv: %v\n", err)
		return 1
	}

	// Advance clock per IR2 for each received message.
	c := a.getClock(agentID)
	var maxTS int64
	for _, e := range events {
		c.Receive(e.LamportTS)
		if e.LamportTS > maxTS {
			maxTS = e.LamportTS
		}
	}
	newTS := c.Value()

	if ag, _ := a.store.GetAgent(agentID); ag != nil {
		_ = a.store.UpdateAgentClock(agentID, newTS, ag.Epoch, ag.Round)
	}
	if maxTS > 0 {
		_ = a.store.SetCursor(agentID, maxTS+1)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"messages": events, "count": len(events), "new_lamport_ts": newTS,
		})
	} else {
		if len(events) == 0 {
			fmt.Println("no new messages")
		} else {
			for _, e := range events {
				body := e.Body
				if *summary && len(body) > 80 {
					body = body[:80] + "..."
				}
				fmt.Printf("[ts=%d] %s: %s\n", e.LamportTS, e.AgentID, body)
			}
			fmt.Fprintf(os.Stderr, "(%d messages, clock now %d)\n", len(events), newTS)
		}
	}
	return 0
}
