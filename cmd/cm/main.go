// Command cm is the clockmail CLI — coordination for concurrent AI agent
// sessions via Lamport clocks and Naiad frontiers.
package main

import (
	"fmt"
	"os"
)

// Set via -ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const (
	defaultDB  = ".clockmail/clockmail.db"
	defaultDir = ".clockmail"
)

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
		fmt.Printf("cm %s (commit %s, built %s)\n", version, commit, date)
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
	case "send", "exchange", "ex":
		os.Exit(a.cmdSend(os.Args[2:]))
	case "broadcast":
		// Shorthand: cm broadcast <message> => cm send all <message>
		os.Exit(a.cmdSend(append([]string{"all"}, os.Args[2:]...)))
	case "recv":
		os.Exit(a.cmdRecv(os.Args[2:]))
	case "lock":
		os.Exit(a.cmdLock(os.Args[2:]))
	case "unlock":
		os.Exit(a.cmdUnlock(os.Args[2:]))
	case "gate":
		os.Exit(a.cmdGate(os.Args[2:]))
	case "review-request", "rr":
		os.Exit(a.cmdReviewRequest(os.Args[2:]))
	case "review-done", "rd":
		os.Exit(a.cmdReviewDone(os.Args[2:]))
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
  send <to> <message>       Send message (drains inbox first, bidirectional)
  broadcast <message>       Send to all agents (shorthand for: send all <msg>)
  recv [--since N] [--summary]  Receive messages (Lamport IR2)
  lock <path> [--ttl N]     Acquire exclusive file lock (total order)
  unlock <path>             Release a file lock
  gate --epoch N [--check]  Block until frontier passes epoch (test gating)
  review-request <commit>   Signal commit ready for review (Lamport causal ordering)
  review-done <commit> <v>  Signal review complete with pass/fail verdict
  frontier [--epoch N]      Check Naiad frontier safety
  log [--since N]           Query the append-only event log
  sync [--epoch N]          Combined: heartbeat + recv + frontier
  watch [--interval N]      Stream messages (or all events with --all)
  status                    Show agent state, locks, frontier overview

Recipients:
  Use "all" as recipient to broadcast to every registered agent (excludes self).
  Example: cm send all "status update"
  Example: cm broadcast "status update"  (equivalent)

Aliases:
  hb        = heartbeat
  ex        = send (formerly exchange)
  exchange  = send (unified, bidirectional by default)
  broadcast = send all
  rr        = review-request
  rd        = review-done

Environment:
  CLOCKMAIL_DB      SQLite database path (default: .clockmail/clockmail.db)
  CLOCKMAIL_AGENT   Default agent ID (avoids passing --agent every time)

All commands support --json for machine-readable output.
All commands support --agent <id> to override CLOCKMAIL_AGENT.

Exit codes:
  0  success
  1  error
  2  lock denied / gate not safe (conflict)
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
