package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/daviddao/clockmail/pkg/frontier"
	"github.com/daviddao/clockmail/pkg/model"
)

// cmdGate blocks until the Naiad frontier passes the specified epoch,
// meaning all agents have advanced past it. This enables test-gate
// coordination: don't test until all agents are done writing.
//
// Usage:
//
//	cm gate --epoch N             # block until epoch N is safe
//	cm gate --epoch N --timeout 5m  # block with timeout
//	cm gate --epoch N --check     # check once, exit 0 if safe, 1 if not
//
// Exit codes:
//
//	0 = epoch is safe to finalize (all agents past it)
//	1 = error or timeout
//	2 = not safe (--check mode only)
func (a *app) cmdGate(args []string) int {
	flags := flag.NewFlagSet("gate", flag.ContinueOnError)
	agent := flags.String("agent", "", "agent ID")
	epoch := flags.Int64("epoch", 0, "epoch to wait for")
	round := flags.Int64("round", 0, "round to wait for")
	timeout := flags.Duration("timeout", 10*time.Minute, "max time to wait")
	interval := flags.Duration("interval", 2*time.Second, "poll interval")
	check := flags.Bool("check", false, "check once and exit (no blocking)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	ts := model.Timestamp{Epoch: *epoch, Round: *round}

	// Single check mode: just test once and exit.
	if *check {
		return a.gateCheck(agentID, ts, *jsonOut)
	}

	// Blocking mode: poll until safe or timeout.
	return a.gateWait(agentID, ts, *timeout, *interval, *jsonOut)
}

func (a *app) gateCheck(agentID string, ts model.Timestamp, jsonOut bool) int {
	active, err := a.store.GetActivePointstamps()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: gate: %v\n", err)
		return 1
	}

	status := frontier.ComputeFrontierStatus(agentID, ts, active)

	if jsonOut {
		printJSON(map[string]interface{}{
			"epoch":         ts.Epoch,
			"round":         ts.Round,
			"safe":          status.SafeToFinalize,
			"blocked_by":    status.BlockedBy,
			"blocker_count": len(status.BlockedBy),
			"active_agents": len(active),
			"mode":          "check",
		})
	} else {
		if status.SafeToFinalize {
			fmt.Printf("SAFE: epoch=%d round=%d — all agents have advanced past this point\n",
				ts.Epoch, ts.Round)
		} else {
			fmt.Printf("NOT SAFE: epoch=%d round=%d\n", ts.Epoch, ts.Round)
			for _, b := range status.BlockedBy {
				fmt.Printf("  blocked by %s at epoch=%d round=%d\n",
					b.AgentID, b.Timestamp.Epoch, b.Timestamp.Round)
			}
		}
	}

	if status.SafeToFinalize {
		return 0
	}
	return 2
}

func (a *app) gateWait(agentID string, ts model.Timestamp, timeout, interval time.Duration, jsonOut bool) int {
	deadline := time.Now().Add(timeout)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	if !jsonOut {
		fmt.Fprintf(os.Stderr, "waiting for epoch=%d round=%d to become safe (timeout=%s, poll=%s)\n",
			ts.Epoch, ts.Round, timeout, interval)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately before first tick.
	if safe := a.checkFrontierSafe(agentID, ts); safe {
		return a.gateSuccess(agentID, ts, jsonOut, time.Duration(0))
	}

	for {
		select {
		case <-sig:
			fmt.Fprintf(os.Stderr, "\ninterrupted\n")
			return 1
		case <-ticker.C:
			if time.Now().After(deadline) {
				if jsonOut {
					printJSON(map[string]interface{}{
						"epoch": ts.Epoch, "round": ts.Round,
						"safe": false, "reason": "timeout",
					})
				} else {
					fmt.Fprintf(os.Stderr, "TIMEOUT: epoch=%d round=%d not safe after %s\n",
						ts.Epoch, ts.Round, timeout)
				}
				return 1
			}

			if safe := a.checkFrontierSafe(agentID, ts); safe {
				elapsed := timeout - time.Until(deadline)
				return a.gateSuccess(agentID, ts, jsonOut, elapsed)
			}
		}
	}
}

func (a *app) checkFrontierSafe(agentID string, ts model.Timestamp) bool {
	active, err := a.store.GetActivePointstamps()
	if err != nil {
		return false
	}
	status := frontier.ComputeFrontierStatus(agentID, ts, active)
	return status.SafeToFinalize
}

func (a *app) gateSuccess(agentID string, ts model.Timestamp, jsonOut bool, elapsed time.Duration) int {
	if jsonOut {
		printJSON(map[string]interface{}{
			"epoch":   ts.Epoch,
			"round":   ts.Round,
			"safe":    true,
			"elapsed": elapsed.String(),
			"mode":    "wait",
		})
	} else {
		fmt.Printf("SAFE: epoch=%d round=%d — all agents have advanced past this point",
			ts.Epoch, ts.Round)
		if elapsed > 0 {
			fmt.Printf(" (waited %s)", elapsed.Round(time.Millisecond))
		}
		fmt.Println()
	}
	return 0
}
