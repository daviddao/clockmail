# Agent Instructions

<!-- BEGIN CLOCKMAIL INTEGRATION -->
## Multi-Agent Coordination with cm (clockmail)

This project uses **cm** for coordinating concurrent AI agent sessions.
Run `cm prime` for current coordination state, or `cm onboard` to get started.

**Quick reference:**
- `cm sync --epoch N`   — Main loop: heartbeat + recv + frontier check
- `cm lock <path>`     — Acquire file lock before editing
- `cm unlock <path>`   — Release when done
- `cm send <to> <msg>` — Send message to another agent
- `cm status`          — Full overview of all agents, locks, frontier

**Environment:** `export CLOCKMAIL_AGENT=<your-id>`

**Session close:** Release all locks and run `cm sync` before ending.
<!-- END CLOCKMAIL INTEGRATION -->
