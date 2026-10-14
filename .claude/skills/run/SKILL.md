---
name: run
description: Launch LeafWiki locally (Go backend + Vite frontend dev server) to manually verify a change in the browser. Use whenever a frontend or backend change needs to be exercised end-to-end instead of just type-checked or unit-tested.
---

# Running LeafWiki locally

LeafWiki needs two processes running side by side: the Go backend (API + serves data)
and the Vite dev server (frontend, proxies `/api`, `/assets`, `/branding` to the backend).

## 1. Backend

```bash
cd cmd/leafwiki
go run . --jwt-secret=dev --admin-password=dev --public-access=true --allow-insecure=true
```

Runs on `http://localhost:8080`. Admin login: username `admin`, password `dev`.
`--public-access=true` lets you view pages without logging in; use the admin login for
anything that touches the tree, settings, or editor.

## 2. Frontend

```bash
cd ui/leafwiki-ui
npm install   # first time only
npm run dev
```

Runs on `http://localhost:5173` — open this URL, not :8080, during development.

## Notes

- Run both in the background (e.g. two `run_in_background` Bash calls) and poll
  `http://localhost:5173` / `http://localhost:8080` readiness before navigating.
- Data is stored on disk under the backend's working directory (`cmd/leafwiki`) by default —
  starting fresh creates an empty wiki with just the root page.
- Before committing, run `npm run format` in `ui/leafwiki-ui` (and in `e2e/` if e2e tests changed).
