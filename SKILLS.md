# Clockmail Skills for AI Agents

Practical patterns for using `cm` to coordinate with other agents on the same codebase.

Run `cm prime` for your live state. Run `cm --help` for command syntax. This document covers **when and why**, not how.

## Session Lifecycle

```
START:   bd ready                    # find your task (determines epoch N)
         cm prime                    # read your state, see who's active
         cm sync --epoch N           # join at the current phase
WORK:    cm lock <file>              # before editing shared files
         cm send <agent> "<msg>"     # coordinate with others
         cm unlock <file>            # when done editing
LOOP:    cm sync --epoch N           # heartbeat + receive + check frontier
END:     cm unlock <all held files>  # release everything
         bd close <task>             # mark beads task done
         cm sync --epoch N+1         # advance epoch, signal you're done
```

## When to Lock

Lock before editing any file another agent might also edit. `cm lock` uses Lamport total ordering to resolve conflicts deterministically — if two agents request the same lock simultaneously, the one with the lower `(timestamp, agent_id)` wins.

- Exit code 0 = you got the lock
- Exit code 2 = denied (someone else holds it)

If denied, either wait and retry, or message the holder asking them to release.

```bash
cm lock auth.go || cm send alice "need auth.go, can you release?"
```

Always unlock when done. Locks have a TTL (default 3600s) but don't rely on expiry.

## When to Send Messages

Message other agents when:
- You've finished work they're waiting on
- You need them to release a lock
- You've found a bug or issue in their area
- You're about to edit something near their work area

Messages are delivered on `cm recv` or `cm sync`. They are not pushed — the recipient must poll.

## Understanding Epochs

An epoch represents a phase of work. The frontier tells you whether all agents have finished a given epoch. If the frontier says `NOT SAFE to finalize epoch=1`, at least one agent is still working in epoch 1. Wait before assuming epoch 1 is complete.

Advance your epoch with `cm heartbeat --epoch N` or `cm sync --epoch N`.

### How to pick your epoch number

When starting a fresh session, run `cm status` to see what epoch other agents are at. Set yours to the same epoch as the current work phase. When you finish a phase and move to the next, increment.

If the project uses beads (`bd`) for issue tracking, use the convention below.

## Epochs with Beads

When beads (`bd`) is available for issue tracking, use this convention to keep epochs in sync across sessions:

### Convention

Epochs are simple integers that you increment as you complete units of work. Start at 0 and advance when you finish a task or phase.

```bash
# Session start: check what others are at, match the highest
cm status                   # see all agents' epochs
cm sync --epoch 3           # join at the current frontier

# Working: stay at your epoch
cm sync --epoch 3           # periodic heartbeat

# Task done: advance
bd close <task-id>          # mark beads task complete
cm sync --epoch 4           # signal you've moved on
```

### How to pick your epoch on session start

1. Run `cm status` — look at the epoch numbers of active agents
2. Pick `max(all agent epochs)` — this puts you at the current frontier
3. If no agents are active, pick the last epoch you see in `cm log` + 1

### How epochs relate to beads tasks

Epochs don't map 1:1 to beads epic IDs (epochs are integers, beads IDs are strings). Instead, treat epochs as a simple counter:

- Each time you finish a beads task and start the next one, increment your epoch
- Each time the beads ready front advances (a `blocks` dependency is satisfied), that's a natural epoch boundary
- The frontier tells you if all agents have moved past an epoch — use this to confirm phase transitions

```bash
bd ready                    # find your next task
bd show <task-id>           # read context
cm sync --epoch 5           # heartbeat at current epoch
# ... work ...
bd close <task-id>          # done
cm sync --epoch 6           # advance
cm frontier --epoch 5       # check: has everyone else moved past 5?
```

### No beads? No problem

Without beads, just use sequential integers and coordinate via messages:

```bash
cm send bob "moving to epoch 2 (tests)"
cm sync --epoch 2
```

## Reading Timestamps

Every event has a logical timestamp `ts`. These are **not** wall-clock times — they're Lamport counters that encode causality.

- `ts=3` means 3 causal steps have happened in this agent's history
- If you see a message `[ts=5] bob: done with tests` and your clock is at 8, your state incorporates bob's message
- Higher ts does not mean "later in real time" — it means "more causal knowledge"

Two events with no causal relationship are **concurrent**. Neither happened "before" the other. This is normal and expected when agents work independently.

## Conflict Resolution

When two agents contend for the same resource (lock), `cm` uses the Lamport total order: compare `(ts, agent_id)` lexicographically. The lower value wins. This is deterministic — every agent reaches the same conclusion without a central coordinator.

## The Sync Command

`cm sync --epoch N` is the main loop command. It does three things:

1. **Heartbeat** — advances your clock, reports your epoch/round
2. **Receive** — fetches new messages from other agents
3. **Frontier** — checks if epoch N is safe to finalize

Run it periodically during your session and always at session end.

## Session Close Protocol

Before ending your session:

1. Release all locks: `cm unlock <path>` for each held file
2. Update beads: `bd close <task>` for completed work, `bd update <id> --status blocked` for incomplete
3. Advance epoch: `cm sync --epoch N+1`
4. Push and sync: `git push && bd sync`
5. Check `cm status` to verify clean state

Failing to unlock leaves files blocked for other agents until TTL expiry.

## Patterns

**Ping-pong coordination:**
```bash
# Agent A
cm lock shared.go && cm send B "editing shared.go"
# ... work ...
cm unlock shared.go && cm send B "shared.go released"
cm sync --epoch 2

# Agent B (after receiving message)
cm lock shared.go  # now succeeds
```

**Waiting for frontier:**
```bash
# Check if all agents finished epoch 1
cm frontier --epoch 1
# SAFE = proceed; NOT SAFE = wait and retry
```

**Batch operations:**
```bash
cm lock auth.go && cm lock session.go   # lock multiple files
# ... work on both ...
cm unlock auth.go && cm unlock session.go && cm sync --epoch 2
```
