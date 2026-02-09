// Command cm is the clockmail CLI — coordination for concurrent AI agent
// sessions via Lamport clocks and Naiad frontiers.
package main

import (
	"fmt"
	"os"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "-h", "help":
		printUsage()
		return
	case "--version", "-v", "version":
		fmt.Println("cm", version)
		return
	}

	a, err := newApp()
	if err != nil {
		fatal("%v", err)
	}
	defer a.Close()

	switch os.Args[1] {
	// Setup
	case "init":
		os.Exit(a.cmdInit(os.Args[2:]))
	case "onboard":
		os.Exit(a.cmdOnboard(os.Args[2:]))
	case "prime":
		os.Exit(a.cmdPrime(os.Args[2:]))

	// Operations
	case "register":
		os.Exit(a.cmdRegister(os.Args[2:]))
	case "heartbeat", "hb":
		os.Exit(a.cmdHeartbeat(os.Args[2:]))
	case "send":
		os.Exit(a.cmdSend(os.Args[2:]))
	case "recv":
		os.Exit(a.cmdRecv(os.Args[2:]))
	case "lock":
		os.Exit(a.cmdLock(os.Args[2:]))
	case "unlock":
		os.Exit(a.cmdUnlock(os.Args[2:]))
	case "frontier":
		os.Exit(a.cmdFrontier(os.Args[2:]))
	case "log":
		os.Exit(a.cmdLog(os.Args[2:]))
	case "sync":
		os.Exit(a.cmdSync(os.Args[2:]))
	case "watch":
		os.Exit(a.cmdWatch(os.Args[2:]))
	case "status":
		os.Exit(a.cmdStatus(os.Args[2:]))

	default:
		fmt.Fprintf(os.Stderr, "cm: unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run 'cm --help' for usage.")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`cm — coordination for concurrent AI agent sessions

Lamport clocks for causal ordering. Naiad frontiers for progress tracking.
Shared SQLite for zero-config communication.

Usage:
  cm <command> [flags]

Setup:
  init [--agent ID]         Initialize clockmail, inject AGENTS.md
  onboard                   Minimal primer for cold-start agents
  prime                     Dynamic coordination context (run at session start)

Commands:
  register <agent_id>       Register an agent session
  heartbeat [--epoch N]     Advance clock, report working position
  send <to> <message>       Send a message (Lamport IR1)
  recv [--since N]          Receive messages (Lamport IR2)
  lock <path> [--ttl N]     Acquire exclusive file lock (total order)
  unlock <path>             Release a file lock
  frontier [--epoch N]      Check Naiad frontier safety
  log [--since N]           Query the append-only event log
  sync [--epoch N]          Combined: heartbeat + recv + frontier
  watch [--interval N]      Stream messages as they arrive
  status                    Show agent state, locks, frontier overview

Aliases:
  hb = heartbeat

Environment:
  CLOCKMAIL_DB      SQLite database path (default: clockmail.db)
  CLOCKMAIL_AGENT   Default agent ID (avoids passing --agent every time)

All commands support --json for machine-readable output.
All commands support --agent <id> to override CLOCKMAIL_AGENT.

Exit codes:
  0  success
  1  error
  2  lock denied (conflict)
`)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "cm: "+format+"\n", args...)
	os.Exit(1)
}
