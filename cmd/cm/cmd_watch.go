package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

func (a *app) cmdWatch(args []string) int {
	flags := flag.NewFlagSet("watch", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID (omit for global stream)")
	all := flags.Bool("all", false, "watch all events from all agents (global mode)")
	kind := flags.String("kind", "", "filter by event kind (msg, lock_req, lock_rel, progress)")
	interval := flags.Int("interval", 1, "poll interval in seconds")
	jsonOut := flags.Bool("json", false, "JSON output (one JSON object per line)")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, agentErr := a.resolveAgent(*agent)
	globalMode := *all || agentErr != nil

	pollInterval := time.Duration(*interval) * time.Second

	// Handle ctrl-c gracefully.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	if globalMode {
		return a.watchGlobal(sig, pollInterval, *kind, *jsonOut)
	}
	return a.watchAgent(sig, agentID, pollInterval, *kind, *jsonOut)
}

// watchGlobal streams all events from all agents. Read-only: no clock
// side-effects, no cursor updates. Safe for passive observers.
func (a *app) watchGlobal(sig chan os.Signal, interval time.Duration, kindFilter string, jsonOut bool) int {
	// Seed cursor to the current max event row ID so we only show new events.
	// We track by row ID (autoincrement) rather than Lamport timestamp
	// because multiple events can share a Lamport timestamp.
	lastSeenID := a.store.MaxEventID()

	kindStr := "all events"
	if kindFilter != "" {
		kindStr = kindFilter + " events"
	}
	fmt.Fprintf(os.Stderr, "watching %s from all agents (poll every %s, ctrl-c to stop)\n",
		kindStr, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\nstopped")
			return 0
		case <-ticker.C:
			events, err := a.store.ListEventsSinceID(lastSeenID, 200)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cm: watch: %v\n", err)
				continue
			}

			for _, e := range events {
				lastSeenID = e.ID

				// Apply kind filter.
				if kindFilter != "" && string(e.Kind) != kindFilter {
					continue
				}

				if jsonOut {
					b, _ := json.Marshal(e)
					fmt.Println(string(b))
				} else {
					printEvent(e)
				}
			}
		}
	}
}

// watchAgent streams messages targeted to a specific agent. Advances the
// agent's Lamport clock (IR2) and updates their cursor.
func (a *app) watchAgent(sig chan os.Signal, agentID string, interval time.Duration, kindFilter string, jsonOut bool) int {
	cursor := a.store.GetCursor(agentID)

	kindStr := "messages"
	if kindFilter != "" {
		kindStr = kindFilter + " events"
	}
	fmt.Fprintf(os.Stderr, "watching %s for %s (poll every %s, ctrl-c to stop)\n",
		kindStr, agentID, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\nstopped")
			return 0
		case <-ticker.C:
			events, err := a.store.ListEventsForAgent(agentID, cursor, 100)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cm: watch: %v\n", err)
				continue
			}

			for _, e := range events {
				// Apply kind filter if set (agent mode only gets msg by default).
				if kindFilter != "" && string(e.Kind) != kindFilter {
					if e.LamportTS >= cursor {
						cursor = e.LamportTS + 1
					}
					continue
				}

				if jsonOut {
					b, _ := json.Marshal(e)
					fmt.Println(string(b))
				} else {
					printEvent(e)
				}
				if e.LamportTS >= cursor {
					cursor = e.LamportTS + 1
				}
			}

			if len(events) > 0 {
				_ = a.store.SetCursor(agentID, cursor)
				c := a.getClock(agentID)
				for _, e := range events {
					c.Receive(e.LamportTS)
				}
				newTS := c.Value()
				if ag, _ := a.store.GetAgent(agentID); ag != nil {
					_ = a.store.UpdateAgentClock(agentID, newTS, ag.Epoch, ag.Round)
				}
			}
		}
	}
}

// printEvent formats an event for human-readable output, matching cm log format.
func printEvent(e model.Event) {
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
