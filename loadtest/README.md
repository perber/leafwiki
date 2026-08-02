# LeafWiki load test — CI regression suite

Self-contained load-test smoke check: builds a throwaway instance, seeds
it, runs a curated set of scenarios, and exits non-zero if anything
errors or regresses past a threshold vs. the checked-in baseline.

**Never point this at a real/production data directory.**

## Run it

```bash
./loadtest/ci-suite.sh
```

Builds the binary, seeds 500 pages, starts and stops its own instance —
nothing needs to be running beforehand. Report lands in
`loadtest/results/ci/<timestamp>/` (`report.md` + `report.json`).

## Update the baseline

After an intentional performance change, or to establish a baseline on a
new machine:

```bash
./loadtest/ci-suite.sh --update-baseline
```

## Options

- `SCENARIO_DURATION` — per-scenario duration (default `8s`)
- `PORT` / `METRICS_PORT` — default `8093` / `9093`
- `--keep-data` — leave the throwaway data dir/binary in place for debugging
- `--regression-threshold-pct` (via `ci-report.py`, default `50`)

**Not yet wired into CI** — run manually until it is. On a shared, noisy
machine, single short runs can vary 20-50%+ from ambient load; re-run
`--update-baseline` on the actual CI runner, ideally idle, before trusting
the threshold to catch real regressions instead of noise.
