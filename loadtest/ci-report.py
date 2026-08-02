#!/usr/bin/env python3
"""ci-report.py — consolidates k6 summary JSONs from a ci-suite.sh run into
a human-readable report + a machine-readable one, and (if a baseline exists)
flags scenarios that regressed beyond a threshold.

Not wired into any CI system yet — this is the local building block for
that, run by hand or via ci-suite.sh until someone adds the workflow file.

Usage:
    ci-report.py <results-dir> [--baseline PATH] [--out-dir PATH]
                 [--regression-threshold-pct N] [--update-baseline]

Exit code: 0 if every scenario has zero errors and (when a baseline exists)
no regression beyond the threshold; 1 otherwise. Intended to gate a CI job
directly via the exit code once wired in.
"""
import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def git_commit() -> str:
    try:
        return subprocess.check_output(
            ["git", "rev-parse", "--short", "HEAD"], text=True, stderr=subprocess.DEVNULL
        ).strip()
    except Exception:
        return "unknown"


def load_scenario(path: Path) -> dict:
    """Extracts the scenario-agnostic metrics every k6 script's summary has
    in common (http_req_duration, http_req_failed, unexpected_errors), so
    this doesn't need to know each scenario's custom Trend metric name."""
    data = json.loads(path.read_text())
    metrics = data.get("metrics", {})

    req_duration = metrics.get("http_req_duration", {})
    req_failed = metrics.get("http_req_failed", {})
    unexpected = metrics.get("unexpected_errors", {})

    return {
        "name": path.stem.replace(".summary", ""),
        "avg_ms": round(req_duration.get("avg", 0.0), 2),
        "p95_ms": round(req_duration.get("p(95)", 0.0), 2),
        "http_req_failed_rate": req_failed.get("value", 0.0),
        "unexpected_errors": int(unexpected.get("count", 0)),
    }


def build_report(results_dir: Path) -> dict:
    summaries = sorted(results_dir.glob("*.summary.json"))
    if not summaries:
        print(f"ci-report.py: no *.summary.json files found in {results_dir}", file=sys.stderr)
        sys.exit(2)

    scenarios = [load_scenario(p) for p in summaries]
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "git_commit": git_commit(),
        "scenarios": scenarios,
    }


def evaluate(report: dict, baseline: dict | None, threshold_pct: float) -> tuple[bool, list[str]]:
    """Returns (ok, findings) — ok is False if anything should fail CI."""
    findings = []
    ok = True

    baseline_by_name = {s["name"]: s for s in baseline["scenarios"]} if baseline else {}

    for s in report["scenarios"]:
        if s["unexpected_errors"] > 0 or s["http_req_failed_rate"] > 0:
            ok = False
            findings.append(
                f"FAIL {s['name']}: {s['unexpected_errors']} unexpected_errors, "
                f"http_req_failed rate {s['http_req_failed_rate']:.4f} (must be 0)"
            )
            continue

        base = baseline_by_name.get(s["name"])
        if base is None:
            findings.append(f"OK   {s['name']}: p95={s['p95_ms']}ms, avg={s['avg_ms']}ms (no baseline to compare)")
            continue

        base_p95 = base["p95_ms"]
        if base_p95 <= 0:
            findings.append(f"OK   {s['name']}: p95={s['p95_ms']}ms (baseline p95 was 0, skipping ratio check)")
            continue

        delta_pct = (s["p95_ms"] - base_p95) / base_p95 * 100
        if delta_pct > threshold_pct:
            ok = False
            findings.append(
                f"REGRESSION {s['name']}: p95 {base_p95}ms -> {s['p95_ms']}ms "
                f"(+{delta_pct:.1f}%, threshold {threshold_pct}%)"
            )
        else:
            sign = "+" if delta_pct >= 0 else ""
            findings.append(f"OK   {s['name']}: p95={s['p95_ms']}ms ({sign}{delta_pct:.1f}% vs baseline)")

    return ok, findings


def write_markdown(report: dict, findings: list[str], ok: bool, out_path: Path) -> None:
    lines = [
        "# LeafWiki Load Test Report",
        "",
        f"- Generated: {report['generated_at']}",
        f"- Commit: `{report['git_commit']}`",
        f"- Result: {'**PASS**' if ok else '**FAIL**'}",
        "",
        "| Scenario | avg (ms) | p95 (ms) | errors | http_req_failed |",
        "|---|--:|--:|--:|--:|",
    ]
    for s in report["scenarios"]:
        lines.append(
            f"| {s['name']} | {s['avg_ms']} | {s['p95_ms']} | {s['unexpected_errors']} | "
            f"{s['http_req_failed_rate']:.4f} |"
        )
    lines += ["", "## Findings", ""]
    lines += [f"- {f}" for f in findings]
    lines.append("")
    out_path.write_text("\n".join(lines))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("results_dir", type=Path, help="Directory containing *.summary.json files from ci-suite.sh")
    parser.add_argument("--baseline", type=Path, default=Path(__file__).parent / "ci-baseline.json")
    parser.add_argument("--out-dir", type=Path, default=None, help="Defaults to results_dir")
    parser.add_argument("--regression-threshold-pct", type=float, default=50.0)
    parser.add_argument("--update-baseline", action="store_true", help="Overwrite the baseline file with this run's results instead of comparing against it")
    args = parser.parse_args()

    out_dir = args.out_dir or args.results_dir
    out_dir.mkdir(parents=True, exist_ok=True)

    report = build_report(args.results_dir)

    if args.update_baseline:
        args.baseline.write_text(json.dumps(report, indent=2) + "\n")
        print(f"ci-report.py: baseline updated -> {args.baseline}")
        sys.exit(0)

    baseline = None
    if args.baseline.exists():
        baseline = json.loads(args.baseline.read_text())
    else:
        print(f"ci-report.py: no baseline at {args.baseline}, reporting without regression checks", file=sys.stderr)

    ok, findings = evaluate(report, baseline, args.regression_threshold_pct)

    report_json_path = out_dir / "report.json"
    report_md_path = out_dir / "report.md"
    report_json_path.write_text(json.dumps(report, indent=2) + "\n")
    write_markdown(report, findings, ok, report_md_path)

    print(report_md_path.read_text())
    print(f"ci-report.py: report written to {report_json_path} and {report_md_path}")

    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
