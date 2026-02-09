package main

import (
	"fmt"
	"time"
)

func (a *app) cmdOnboard(_ []string) int {
	agentID := a.agentID
	dbPath := envOr("CLOCKMAIL_DB", defaultDB)

	fmt.Println("cm (clockmail) — multi-agent coordination via Lamport clocks + Naiad frontiers")
	fmt.Println()

	if agentID != "" {
		fmt.Printf("  Your agent ID:  %s (from CLOCKMAIL_AGENT)\n", agentID)
	} else {
		fmt.Println("  Your agent ID:  (not set — export CLOCKMAIL_AGENT=<id>)")
	}
	fmt.Printf("  Database:       %s\n", dbPath)
	fmt.Println()

	agents, _ := a.store.ListAgents()
	if len(agents) > 0 {
		fmt.Printf("  Active agents:  %d\n", len(agents))
		for _, ag := range agents {
			stale := ""
			if time.Since(ag.LastSeen) > 10*time.Minute {
				stale = " (stale)"
			}
			marker := ""
			if ag.ID == agentID {
				marker = " <-- you"
			}
			fmt.Printf("    %-15s epoch=%d round=%d%s%s\n",
				ag.ID, ag.Epoch, ag.Round, stale, marker)
		}
		fmt.Println()
	}

	fmt.Println("Run 'cm prime' for full coordination context.")
	fmt.Println("Run 'cm --help' for all commands.")
	fmt.Println("Run 'cm status' for a detailed overview.")

	return 0
}
