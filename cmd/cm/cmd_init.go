package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	agentsBeginMarker = "<!-- BEGIN CLOCKMAIL INTEGRATION -->"
	agentsEndMarker   = "<!-- END CLOCKMAIL INTEGRATION -->"
)

const agentsSection = `<!-- BEGIN CLOCKMAIL INTEGRATION -->
## Multi-Agent Coordination with cm (clockmail)

This project uses **cm** for coordinating concurrent AI agent sessions.
Run ` + "`cm prime`" + ` for current coordination state, or ` + "`cm onboard`" + ` to get started.

**Quick reference:**
- ` + "`cm sync --epoch N`" + `   — Main loop: heartbeat + recv + frontier check
- ` + "`cm lock <path>`" + `     — Acquire file lock before editing
- ` + "`cm unlock <path>`" + `   — Release when done
- ` + "`cm send <to> <msg>`" + ` — Send message to another agent
- ` + "`cm status`" + `          — Full overview of all agents, locks, frontier

**Environment:** ` + "`export CLOCKMAIL_AGENT=<your-id>`" + `

**Session close:** Release all locks and run ` + "`cm sync`" + ` before ending.
<!-- END CLOCKMAIL INTEGRATION -->
`

func (a *app) cmdInit(args []string) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID to register (optional)")
	agentsFile := flags.String("agents-md", "AGENTS.md", "path to AGENTS.md")
	skipAgents := flags.Bool("skip-agents-md", false, "don't touch AGENTS.md")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	dbPath := envOr("CLOCKMAIL_DB", defaultDB)

	agents, err := a.store.ListAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: init: database error: %v\n", err)
		return 1
	}

	fmt.Printf("initialized clockmail (db: %s)\n", dbPath)
	if len(agents) > 0 {
		fmt.Printf("  %d existing agent(s)\n", len(agents))
	}

	agentID := *agent
	if agentID == "" {
		agentID = a.agentID
	}
	if agentID != "" {
		ag, err := a.store.RegisterAgent(agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cm: init: register: %v\n", err)
			return 1
		}
		fmt.Printf("  registered agent %q (clock=%d)\n", ag.ID, ag.Clock)
	}

	if !*skipAgents {
		if err := injectAgentsSection(*agentsFile); err != nil {
			fmt.Fprintf(os.Stderr, "cm: AGENTS.md: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println("next steps:")
	if agentID == "" {
		fmt.Println("  export CLOCKMAIL_AGENT=<your-id>")
		fmt.Println("  cm register <your-id>")
	} else {
		fmt.Printf("  export CLOCKMAIL_AGENT=%s\n", agentID)
	}
	fmt.Println("  cm prime       # see coordination context")
	fmt.Println("  cm sync        # main loop command")

	return 0
}

// injectAgentsSection creates or updates AGENTS.md with the clockmail section.
// Uses HTML markers for idempotent updates.
func injectAgentsSection(path string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		newContent := "# Agent Instructions\n\n" + agentsSection
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		fmt.Printf("  created %s with clockmail section\n", path)
		return nil
	} else if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	text := string(content)

	if strings.Contains(text, agentsBeginMarker) {
		start := strings.Index(text, agentsBeginMarker)
		end := strings.Index(text, agentsEndMarker)
		if start >= 0 && end >= 0 {
			endOfMarker := end + len(agentsEndMarker)
			if nl := strings.Index(text[endOfMarker:], "\n"); nl >= 0 {
				endOfMarker += nl + 1
			}
			newContent := text[:start] + agentsSection + text[endOfMarker:]
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("update %s: %w", path, err)
			}
			fmt.Printf("  updated clockmail section in %s\n", path)
			return nil
		}
	}

	newContent := text
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n" + agentsSection
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("update %s: %w", path, err)
	}
	fmt.Printf("  added clockmail section to %s\n", path)
	return nil
}
