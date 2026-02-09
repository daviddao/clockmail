package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdLog(args []string) int {
	flags := flag.NewFlagSet("log", flag.ContinueOnError)
	sinceTS := flags.Int64("since", 0, "fetch events with lamport_ts >= this")
	limit := flags.Int("limit", 50, "max events to return")
	kind := flags.String("kind", "", "filter by event kind")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	events, err := a.store.ListEvents(*sinceTS, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: log: %v\n", err)
		return 1
	}

	if *kind != "" {
		filtered := events[:0]
		for _, e := range events {
			if string(e.Kind) == *kind {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	if *jsonOut {
		printJSON(map[string]interface{}{"events": events, "count": len(events)})
	} else {
		if len(events) == 0 {
			fmt.Println("no events")
		} else {
			for _, e := range events {
				switch e.Kind {
				case model.EventMsg:
					fmt.Printf("[ts=%d] %s -> %s: %s\n", e.LamportTS, e.AgentID, e.Target, e.Body)
				case model.EventLockReq:
					fmt.Printf("[ts=%d] %s lock-req %s\n", e.LamportTS, e.AgentID, e.Target)
				case model.EventLockRel:
					fmt.Printf("[ts=%d] %s unlock %s\n", e.LamportTS, e.AgentID, e.Target)
				case model.EventProgress:
					fmt.Printf("[ts=%d] %s heartbeat epoch=%d round=%d\n",
						e.LamportTS, e.AgentID, e.Epoch, e.Round)
				default:
					fmt.Printf("[ts=%d] %s %s %s %s\n",
						e.LamportTS, e.AgentID, e.Kind, e.Target, e.Body)
				}
			}
		}
	}
	return 0
}
