package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/daviddao/clockmail/internal/frontier"
	"github.com/daviddao/clockmail/internal/model"
)

func (a *app) cmdFrontier(args []string) int {
	flags := flag.NewFlagSet("frontier", flag.ContinueOnError)
	agent := flags.String("agent", "", "requesting agent ID")
	epoch := flags.Int64("epoch", 0, "epoch to check safety for")
	round := flags.Int64("round", 0, "round to check safety for")
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
	active, err := a.store.GetActivePointstamps()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: frontier: %v\n", err)
		return 1
	}

	status := frontier.ComputeFrontierStatus(agentID, ts, active)

	if *jsonOut {
		printJSON(status)
	} else {
		if status.SafeToFinalize {
			fmt.Printf("SAFE to finalize epoch=%d round=%d\n", *epoch, *round)
		} else {
			fmt.Printf("NOT SAFE to finalize epoch=%d round=%d\n", *epoch, *round)
			for _, b := range status.BlockedBy {
				fmt.Printf("  blocked by %s at epoch=%d round=%d\n",
					b.AgentID, b.Timestamp.Epoch, b.Timestamp.Round)
			}
		}
		if len(status.Frontier) > 0 {
			fmt.Println("frontier:")
			for _, p := range status.Frontier {
				fmt.Printf("  %s @ epoch=%d round=%d\n",
					p.AgentID, p.Timestamp.Epoch, p.Timestamp.Round)
			}
		}
	}
	return 0
}
