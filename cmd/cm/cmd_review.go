package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

// reviewPayload is the structured metadata embedded in review event bodies.
// It is JSON-encoded into the event body so that machines can parse it,
// while the plain-text fallback in cm recv remains human-readable.
type reviewPayload struct {
	Type    string   `json:"type"`              // "review-request" or "review-done"
	Commit  string   `json:"commit"`            // git commit SHA (short or full)
	Files   []string `json:"files,omitempty"`   // affected files (request only)
	Verdict string   `json:"verdict,omitempty"` // "pass" or "fail" (done only)
	Comment string   `json:"comment,omitempty"` // optional reviewer comment
}

// cmdReviewRequest signals that a commit is ready for review. It sends a
// structured message to a reviewer (default: "tester") carrying the commit
// SHA and optionally the list of affected files.
//
// The Lamport timestamp on the resulting event establishes the causal
// "happened-before" anchor: any subsequent review-done event will have a
// strictly higher Lamport timestamp (by IR2), proving review-after-write.
//
// Usage:
//
//	cm review-request <commit> [files...]            # send to tester
//	cm review-request <commit> --to all [files...]   # broadcast
//	cm review-request <commit> --to planner f1 f2    # specific reviewer
func (a *app) cmdReviewRequest(args []string) int {
	flags := flag.NewFlagSet("review-request", flag.ContinueOnError)
	agent := flags.String("agent", "", "sender agent ID")
	to := flags.String("to", "tester", "reviewer agent ID (default: tester)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: cm review-request <commit> [files...] [--to reviewer] [--json]")
		fmt.Fprintln(os.Stderr, "  Signals a commit is ready for review. Sends structured message to reviewer.")
		fmt.Fprintln(os.Stderr, "  The Lamport timestamp proves causal ordering: review happens-after commit.")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	commitSHA := flags.Arg(0)
	var files []string
	if flags.NArg() > 1 {
		files = flags.Args()[1:]
	}

	ep, rn := a.resolveEpochRound(agentID, -1, -1)
	c := a.getClock(agentID)

	// Drain inbox (Lamport IR2) before sending.
	inbox := a.drainInbox(agentID, c)
	printInbox(inbox)

	// Build structured payload.
	payload := reviewPayload{
		Type:   "review-request",
		Commit: commitSHA,
		Files:  files,
	}
	bodyBytes, _ := json.Marshal(payload)

	// Tick and send (Lamport IR1).
	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, ep, rn)

	recipients, err := a.resolveRecipients(*to, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: review-request: %v\n", err)
		return 1
	}

	var eventIDs []int64
	for _, r := range recipients {
		id, err := a.store.InsertEvent(&model.Event{
			AgentID:   agentID,
			LamportTS: ts,
			Epoch:     ep,
			Round:     rn,
			Kind:      model.EventReviewReq,
			Target:    r,
			Body:      string(bodyBytes),
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "cm: review-request: %v\n", err)
			return 1
		}
		eventIDs = append(eventIDs, id)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"lamport_ts": ts,
			"event_ids":  eventIDs,
			"commit":     commitSHA,
			"files":      files,
			"recipients": recipients,
			"type":       "review-request",
		})
	} else {
		fileStr := ""
		if len(files) > 0 {
			fileStr = fmt.Sprintf(" files=[%s]", strings.Join(files, ", "))
		}
		fmt.Printf("review-request sent to %s at ts=%d commit=%s%s\n",
			strings.Join(recipients, ","), ts, commitSHA, fileStr)
	}
	return 0
}

// cmdReviewDone signals that a review is complete. The reviewer sends
// a structured verdict (pass/fail) back to the original author or to all.
//
// Because the reviewer received the review-request first (advancing their
// clock via IR2), the review-done event's Lamport timestamp is guaranteed
// to be strictly greater than the review-request's timestamp. This is the
// provable happened-after relationship from Lamport (1978).
//
// Usage:
//
//	cm review-done <commit> pass                # approve
//	cm review-done <commit> fail "needs fix"    # reject with comment
//	cm review-done <commit> pass --to sergie    # specific author
func (a *app) cmdReviewDone(args []string) int {
	flags := flag.NewFlagSet("review-done", flag.ContinueOnError)
	agent := flags.String("agent", "", "reviewer agent ID")
	to := flags.String("to", "all", "author agent ID to notify (default: all)")
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: cm review-done <commit> <pass|fail> [comment] [--to author] [--json]")
		fmt.Fprintln(os.Stderr, "  Signals a review is complete with a verdict.")
		fmt.Fprintln(os.Stderr, "  Lamport timestamp is guaranteed > review-request (proves review-after-write).")
		return 1
	}

	agentID, err := a.resolveAgent(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: %v\n", err)
		return 1
	}

	commitSHA := flags.Arg(0)
	verdict := strings.ToLower(flags.Arg(1))
	if verdict != "pass" && verdict != "fail" {
		fmt.Fprintf(os.Stderr, "cm: review-done: verdict must be 'pass' or 'fail', got %q\n", verdict)
		return 1
	}
	var comment string
	if flags.NArg() > 2 {
		comment = strings.Join(flags.Args()[2:], " ")
	}

	ep, rn := a.resolveEpochRound(agentID, -1, -1)
	c := a.getClock(agentID)

	// Drain inbox (Lamport IR2) before sending.
	inbox := a.drainInbox(agentID, c)
	printInbox(inbox)

	// Build structured payload.
	payload := reviewPayload{
		Type:    "review-done",
		Commit:  commitSHA,
		Verdict: verdict,
		Comment: comment,
	}
	bodyBytes, _ := json.Marshal(payload)

	// Tick and send (Lamport IR1).
	ts := c.Tick()
	_ = a.store.UpdateAgentClock(agentID, ts, ep, rn)

	recipients, err := a.resolveRecipients(*to, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: review-done: %v\n", err)
		return 1
	}

	var eventIDs []int64
	for _, r := range recipients {
		id, err := a.store.InsertEvent(&model.Event{
			AgentID:   agentID,
			LamportTS: ts,
			Epoch:     ep,
			Round:     rn,
			Kind:      model.EventReviewDone,
			Target:    r,
			Body:      string(bodyBytes),
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "cm: review-done: %v\n", err)
			return 1
		}
		eventIDs = append(eventIDs, id)
	}

	if *jsonOut {
		printJSON(map[string]interface{}{
			"lamport_ts": ts,
			"event_ids":  eventIDs,
			"commit":     commitSHA,
			"verdict":    verdict,
			"comment":    comment,
			"recipients": recipients,
			"type":       "review-done",
		})
	} else {
		commentStr := ""
		if comment != "" {
			commentStr = fmt.Sprintf(" comment=%q", comment)
		}
		fmt.Printf("review-done sent to %s at ts=%d commit=%s verdict=%s%s\n",
			strings.Join(recipients, ","), ts, commitSHA, verdict, commentStr)
	}
	return 0
}
