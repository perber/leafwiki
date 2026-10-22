---
name: run
description: Launch LeafWiki locally (Go backend + Vite frontend dev server) to manually verify a change in the browser. Use whenever a frontend or backend change needs to be exercised end-to-end instead of just type-checked or unit-tested.
---

# Running LeafWiki locally

LeafWiki needs two processes running side by side: the Go backend (API + serves data)
and the Vite dev server (frontend, proxies `/api`, `/assets`, `/branding` to the backend).

## Start

```bash
make setup   # first time, or after package-lock.json changed
make dev
```

`make dev` builds and starts the backend on `http://localhost:8080` and Vite on
`http://localhost:5173` — open :5173, not :8080, during development. Admin login: username
`admin`, password `devpassword` (the minimum is 8 characters). `--public-access=true` lets you
view pages without logging in; use the admin login for anything that touches the tree,
settings, or editor. Extra backend flags: `make dev ARGS=--enable-revision=true`. Ctrl-C, or
SIGTERM to the script, stops both.

Without `make dev`: `go run . --jwt-secret=dev --admin-password=devpassword
--public-access=true --allow-insecure=true` in `cmd/leafwiki`, and `npm run dev` in
`ui/leafwiki-ui`.

## Notes

- Run `make dev` in the background (a `run_in_background` Bash call) and poll
  `http://localhost:5173` / `http://localhost:8080` readiness before navigating.
  Stop it with `pkill -TERM -f scripts/dev.sh`: a background job ignores SIGINT, and the
  script's trap stops both servers on SIGTERM.
- Click-throughs and screenshots belong in a subagent that reports back a short text verdict.
  Every image read in the main session stays in its context and is paid for again on every
  later turn.
- Data is stored on disk under the backend's working directory (`cmd/leafwiki`) by default —
  starting fresh creates an empty wiki with just the root page.
- Before committing, run `npm run format` in `ui/leafwiki-ui` (and in `e2e/` if e2e tests changed).
