#!/usr/bin/env python3

import argparse
import json
from collections import Counter
from pathlib import Path


SEVERITY_ORDER = ("Critical", "High", "Medium", "Low", "Negligible", "Unknown")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate a Shields-compatible badge payload from Grype JSON output."
    )
    parser.add_argument(
        "--input",
        default="grype-results.json",
        help="Path to the Grype JSON results file.",
    )
    parser.add_argument(
        "--output",
        required=True,
        help="Path to write the Shields endpoint JSON payload.",
    )
    parser.add_argument(
        "--image",
        default="yiucloud/aerodocs:latest",
        help="Image name to report in console output.",
    )
    return parser.parse_args()


def load_counts(results_path: Path) -> tuple[Counter, str]:
    if not results_path.exists():
        return Counter(), "scan unavailable"

    try:
        data = json.loads(results_path.read_text())
    except json.JSONDecodeError:
        return Counter(), "scan error"

    counts: Counter = Counter()
    for match in data.get("matches", []):
        severity = (match.get("vulnerability") or {}).get("severity") or "Unknown"
        normalized = severity.title()
        if normalized not in SEVERITY_ORDER:
            normalized = "Unknown"
        counts[normalized] += 1

    return counts, "ok"


def badge_color(counts: Counter, status: str) -> str:
    if status != "ok":
        return "lightgrey"
    if counts["Critical"] > 0:
        return "red"
    if counts["High"] > 0:
        return "orange"
    if counts["Medium"] > 0:
        return "yellow"
    if counts["Low"] > 0 or counts["Negligible"] > 0:
        return "blue"
    if counts["Unknown"] > 0:
        return "lightgrey"
    return "brightgreen"


def badge_message(counts: Counter, status: str) -> str:
    if status != "ok":
        return status

    message = [
        f"C:{counts['Critical']}",
        f"H:{counts['High']}",
        f"M:{counts['Medium']}",
        f"L:{counts['Low']}",
    ]

    if counts["Unknown"] > 0:
        message.append(f"U:{counts['Unknown']}")

    return " ".join(message)


def main() -> int:
    args = parse_args()
    counts, status = load_counts(Path(args.input))

    payload = {
        "schemaVersion": 1,
        "label": "docker vulns",
        "message": badge_message(counts, status),
        "color": badge_color(counts, status),
        "namedLogo": "docker",
    }

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, indent=2) + "\n")

    print(
        f"{args.image}: {payload['message']} "
        f"(critical={counts['Critical']}, high={counts['High']}, "
        f"medium={counts['Medium']}, low={counts['Low']}, unknown={counts['Unknown']})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
