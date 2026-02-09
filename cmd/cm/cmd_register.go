package main

import (
	"flag"
	"fmt"
	"os"
)

func (a *app) cmdRegister(args []string) int {
	flags := flag.NewFlagSet("register", flag.ContinueOnError)
	jsonOut := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: cm register <agent_id> [--json]")
		return 1
	}

	agent, err := a.store.RegisterAgent(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cm: register: %v\n", err)
		return 1
	}

	if *jsonOut {
		printJSON(agent)
	} else {
		fmt.Printf("registered agent %q (clock=%d, epoch=%d, round=%d)\n",
			agent.ID, agent.Clock, agent.Epoch, agent.Round)
		fmt.Fprintf(os.Stderr, "hint: export CLOCKMAIL_AGENT=%s\n", agent.ID)
	}
	return 0
}
