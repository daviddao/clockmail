package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (a *app) cmdWatch(args []string) int {
	flags := flag.NewFlagSet("watch", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID")
	interval := flags.Int("interval", 1, "poll interval in seconds")
	jsonOut := flags.Bool("json", false, "JSON output (one JSON object per line)")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	cursor := a.store.GetCursor(agentID)
	pollInterval := time.Duration(*interval) * time.Second

	// Handle ctrl-c gracefully.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Fprintf(os.Stderr, "watching messages for %s (poll every %s, ctrl-c to stop)\n",
		agentID, pollInterval)

	ticker := time.NewTicker(pollInterval)
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
				if *jsonOut {
					b, _ := json.Marshal(e)
					fmt.Println(string(b))
				} else {
					fmt.Printf("[ts=%d] %s: %s\n", e.LamportTS, e.AgentID, e.Body)
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
