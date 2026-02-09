package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

// cmdSend is the unified send command. It drains the inbox before sending
// (bidirectional by default). Use --quiet to suppress inbox output.
//
// The old "exchange" command is now an alias for "send" (see main.go).
// The special recipient "all" broadcasts to every registered agent.
//
// Usage: cm send <to> <message> [--quiet] [--agent ID] [--json]
func (a *app) cmdSend(args []string) int {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	agent := flags.String("agent", "", "sender agent ID")
	epoch := flags.Int64("epoch", -1, "epoch context (-1 = keep current)")
	round := flags.Int64("round", -1, "round context (-1 = keep current)")
	quiet := flags.Bool("quiet", false, "suppress inbox output (fire-and-forget mode)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: cm send <to> <message> [--quiet] [--agent ID] [--json]")
		fmt.Fprintln(os.Stderr, "  Sends a message after draining your inbox (bidirectional by default).")
		fmt.Fprintln(os.Stderr, "  Use 'all' as recipient to broadcast to every registered agent.")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	ep, rn := a.resolveEpochRound(agentID, *epoch, *round)
	to := flags.Arg(0)
	body := strings.Join(flags.Args()[1:], " ")

	c := a.getClock(agentID)

	// Step 1: Drain inbox (Lamport IR2). Always drain; output depends on flags.
	inbox := a.drainInbox(agentID, c)
	if !*jsonOut {
		if *quiet {
			// Quiet mode: inbox to stderr (old send behavior).
			printInbox(inbox)
		} else {
			// Default: inbox to stdout (old exchange behavior).
			if len(inbox) > 0 {
				fmt.Printf("=== %d received message(s) ===\n", len(inbox))
				for _, e := range inbox {
					msgBody := e.Body
					if len(msgBody) > 120 {
						msgBody = msgBody[:120] + "..."
					}
					fmt.Printf("  [ts=%d] %s: %s\n", e.LamportTS, e.AgentID, msgBody)
				}
				fmt.Println("============================")
				fmt.Println()
			} else {
				fmt.Println("(no pending messages)")
				fmt.Println()
			}
		}
	}

	// Step 2: Send (Lamport IR1).
	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, ep, rn)

	recipients, err := a.resolveRecipients(to, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: send: %v\n", err)
		return 1
	}
	var eventIDs []int64
	for _, r := range recipients {
		id, err := a.store.InsertEvent(&model.Event{
			AgentID:   agentID,
			LamportTS: ts,
			Epoch:     ep,
			Round:     rn,
			Kind:      model.EventMsg,
			Target:    r,
			Body:      body,
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "cm: send: %v\n", err)
			return 1
		}
		eventIDs = append(eventIDs, id)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"lamport_ts":  ts,
			"event_ids":   eventIDs,
			"recipients":  len(eventIDs),
			"broadcast":   strings.EqualFold(strings.TrimSpace(to), "all"),
			"inbox":       inbox,
			"inbox_count": len(inbox),
		})
	} else {
		recipientNames := strings.Join(recipients, ",")
		if strings.EqualFold(strings.TrimSpace(to), "all") {
			fmt.Printf("broadcast to %s at ts=%d (%d recipients)\n", recipientNames, ts, len(eventIDs))
		} else {
			fmt.Printf("sent to %s at ts=%d (%d recipients)\n", recipientNames, ts, len(eventIDs))
		}
	}
	return 0
}
