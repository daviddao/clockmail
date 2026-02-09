package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdRecv(args []string) int {
	flags := flag.NewFlagSet("recv", flag.ContinueOnError)
	agent := flags.String("agent", "", "recipient agent ID")
	sinceTS := flags.Int64("since", -1, "fetch events with lamport_ts >= this (-1 = use cursor)")
	limit := flags.Int("limit", 100, "max messages to return")
	from := flags.String("from", "", "filter messages by sender agent ID")
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

	// Advance clock per IR2 for ALL received messages, even if we filter
	// the display. This is correct per Lamport 1978: the agent's clock
	// must advance past all messages it has seen, regardless of display
	// filtering. Filtering is a presentation concern, not a clock concern.
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

	// Apply --from filter for display (after clock advancement).
	displayed := events
	if *from != "" {
		displayed = filterByFrom(events, *from)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"messages":       displayed,
			"count":          len(displayed),
			"total_received": len(events),
			"new_lamport_ts": newTS,
		})
	} else {
		if len(events) == 0 {
			fmt.Println("no new messages")
		} else if len(displayed) == 0 {
			fmt.Fprintf(os.Stderr, "(%d messages received, none from %q, clock now %d)\n",
				len(events), *from, newTS)
		} else {
			for _, e := range displayed {
				body := e.Body
				if *summary && len(body) > 80 {
					body = body[:80] + "..."
				}
				fmt.Printf("[ts=%d] %s: %s\n", e.LamportTS, e.AgentID, body)
			}
			if *from != "" && len(displayed) < len(events) {
				fmt.Fprintf(os.Stderr, "(%d shown from %q, %d total received, clock now %d)\n",
					len(displayed), *from, len(events), newTS)
			} else {
				fmt.Fprintf(os.Stderr, "(%d messages, clock now %d)\n", len(events), newTS)
			}
		}
	}
	return 0
}

// filterByFrom returns only events sent by the given agent ID.
func filterByFrom(events []model.Event, from string) []model.Event {
	var filtered []model.Event
	for _, e := range events {
		if e.AgentID == from {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
