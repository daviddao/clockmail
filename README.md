# Agent Clockmail

Coordinate multiple AI agent sessions working on the same codebase. Agents communicate through a shared SQLite database using the `cm` CLI.

## Install

One-liner (requires `go` and `git`):

```bash
curl -sSL https://raw.githubusercontent.com/daviddao/clockmail/main/install.sh | sh
```

Custom install directory:

```bash
curl -sSL https://raw.githubusercontent.com/daviddao/clockmail/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

Or build from source manually:

```bash
git clone https://github.com/daviddao/clockmail.git && cd clockmail
go build -ldflags "-X main.version=$(git describe --tags 2>/dev/null || git rev-parse --short HEAD) -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o cm ./cmd/cm
```

## Quick Start

```bash
cm init --agent alice        # create DB, register yourself, inject AGENTS.md
export CLOCKMAIL_AGENT=alice

cm register bob              # register another agent
cm send bob "working on auth.go, don't touch it"
cm lock auth.go              # exclusive file lock
# ... do work ...
cm unlock auth.go
cm sync --epoch 1            # heartbeat + receive messages + check frontier
```

## How It Works

### Lamport Timestamps

Every event (message, heartbeat, lock request) gets a logical timestamp `ts`. There is no wall clock — just a counter that establishes **causality**.

**Rules** (from Lamport 1978):
- **IR1**: Before each local event, increment your clock: `ts = ts + 1`
- **IR2**: When sending a message, attach your current `ts`. When receiving a message with timestamp `T`, set `ts = max(ts, T) + 1`

This guarantees: if event A causally precedes event B, then `ts(A) < ts(B)`. Two events with no causal link are **concurrent** — neither happened "before" the other.

**Total ordering for conflicts**: When two agents request the same lock, the one with the lower `(ts, agent_id)` wins. The agent ID breaks ties deterministically. This is why `cm lock` can resolve conflicts without a central coordinator.

**Reading `cm` output**: When you see `ts=5`, that means 5 causal steps have occurred in this agent's history. A message showing `[ts=3] alice: ...` was sent when alice's clock read 3. If your clock is at 7, you know your current state incorporates alice's message (since receiving it bumped your clock past 3).

### Naiad Frontiers

Each agent reports a working position as `(epoch, round)` — called a **pointstamp**. The **frontier** is the set of minimum active pointstamps across all agents.

**What this answers**: "Can I safely assume all agents are done with epoch N?" If every agent has advanced past epoch N, the frontier has moved past it, and it is **SAFE** to finalize. If any agent is still at or behind epoch N, it is **NOT SAFE**.

**Reading frontier output**:
- `SAFE to finalize epoch=1` — all agents moved past epoch 1, no late-arriving work is possible
- `NOT SAFE ... blocked by bob at epoch=1` — bob hasn't advanced yet, wait or coordinate

Use `cm frontier --epoch N` to check, or `cm sync --epoch N` which checks automatically.

## Commands

| Command | What it does |
|---------|-------------|
| `cm init [--agent ID]` | Create DB, register agent, inject AGENTS.md |
| `cm onboard` | Print a short primer (for cold-start agents reading AGENTS.md) |
| `cm prime` | Print full coordination context: your state, peers, locks, frontier |
| `cm register <id>` | Register a new agent |
| `cm heartbeat [--epoch N]` | Advance clock, report working position |
| `cm send <to> <msg>` | Send message (comma-separated recipients) |
| `cm recv [--summary]` | Receive new messages (cursor-tracked, only shows unread; `--summary` truncates to 80 chars) |
| `cm lock <path>` | Acquire exclusive file lock (exit 2 if denied) |
| `cm unlock <path>` | Release file lock |
| `cm frontier [--epoch N]` | Check if epoch N is safe to finalize |
| `cm log` | Show all events in causal order |
| `cm sync [--epoch N]` | Combined: heartbeat + recv + frontier |
| `cm watch` | Stream messages as they arrive (blocks until Ctrl-C) |
| `cm status` | Overview of all agents, locks, and frontier |

All commands accept `--agent <id>` (overrides `CLOCKMAIL_AGENT`) and `--json` for machine-readable output. `hb` is an alias for `heartbeat`.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `CLOCKMAIL_DB` | `.clockmail/clockmail.db` | Path to shared SQLite database |
| `CLOCKMAIL_AGENT` | *(none)* | Your agent ID (avoids `--agent` on every call) |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |
| 2 | Lock denied (another agent holds it) |

## Agent Integration Pattern

`cm init` injects a small section into `AGENTS.md` that tells new agents to run `cm onboard`. This follows progressive disclosure:

```
AGENTS.md  -->  "run cm onboard"
cm onboard -->  who you are, who's active, what to do next
cm prime   -->  full dynamic context (run this at session start)
cm status  -->  detailed runtime view
```

See [SKILL.md](SKILL.md) for agent workflow patterns: when to lock, how to read timestamps, session lifecycle, and conflict resolution.

## Example Session

```bash
# Terminal 1: Alice
cm init --agent alice && export CLOCKMAIL_AGENT=alice
cm heartbeat --epoch 1
cm lock auth.go
cm send bob "editing auth.go"
cm unlock auth.go
cm heartbeat --epoch 2

# Terminal 2: Bob
export CLOCKMAIL_AGENT=bob
cm register bob
cm sync --epoch 1
# => receives alice's message, sees frontier is SAFE (alice moved to epoch 2)
```
