# Agent Instructions

<!-- BEGIN CLOCKMAIL INTEGRATION -->
## Multi-Agent Coordination with cm (clockmail)

This project uses **cm** for coordinating concurrent AI agent sessions.
Run `cm prime` for current coordination state, or `cm onboard` to get started.

**Quick reference:**
- `cm sync --epoch N`      — Main loop: heartbeat + recv + frontier check
- `cm send <to> <msg>`    — Send message (drains inbox first, bidirectional)
- `cm send all <msg>`     — Broadcast to all agents
- `cm broadcast <msg>`    — Same as send all
- `cm lock <path>`        — Acquire file lock (auto-receives inbox first)
- `cm unlock <path>`      — Release when done
- `cm recv`               — Check inbox explicitly
- `cm status`             — Full overview of all agents, locks, frontier

**Environment:** `export CLOCKMAIL_AGENT=<your-id>`

**Session close:** Release all locks and run `cm sync` before ending.
<!-- END CLOCKMAIL INTEGRATION -->
