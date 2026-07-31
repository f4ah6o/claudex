#!/usr/bin/env python3
"""Prepare a reviewable, ownership-aware upstream synchronization report."""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def run(root: Path, *args: str, check: bool = True) -> str:
    result = subprocess.run(
        ["git", *args], cwd=root, text=True, capture_output=True, check=False
    )
    if check and result.returncode:
        raise RuntimeError(result.stderr.strip() or "git command failed")
    return result.stdout


def matches(path: str, pattern: str) -> bool:
    return fnmatch.fnmatchcase(path, pattern) or (
        pattern.endswith("/**") and path.startswith(pattern[:-3])
    )


def load_manifest(path: Path) -> dict:
    with path.open(encoding="utf-8") as stream:
        manifest = json.load(stream)
    if manifest.get("schema_version") != 1:
        raise RuntimeError("unsupported upstream manifest schema")
    return manifest


def classify(manifest: dict, paths: list[str]) -> tuple[dict[str, str], list[str]]:
    ownership: dict[str, str] = {}
    uncovered: list[str] = []
    excluded = manifest.get("excluded", [])
    for path in paths:
        if any(matches(path, pattern) for pattern in excluded):
            continue
        owner = None
        for entry in manifest.get("ownership", []):
            if any(matches(path, pattern) for pattern in entry.get("paths", [])):
                owner = entry.get("owner")
                break
        if owner is None:
            uncovered.append(path)
        else:
            ownership[path] = owner
    return ownership, uncovered


def digest(path: Path) -> str | None:
    if not path.is_file():
        return None
    hasher = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def classify_change(path: str) -> list[str]:
    findings: list[str] = []
    if path in {"go.mod", "go.sum"}:
        findings.append("dependency change")
    if any(token in path.lower() for token in ("route", "provider", "plugin", "management", "wsrelay")):
        findings.append("route/provider/plugin surface")
    if any(token in path.lower() for token in ("auth", "crypto", "security", "dockerfile", "workflow")):
        findings.append("security-sensitive path")
    return findings


def workspace_paths(root: Path, upstream_root: Path) -> tuple[list[str], list[str]]:
    current = set(run(root, "ls-files").splitlines())
    incoming = set(run(upstream_root, "ls-files").splitlines())
    return sorted(current | incoming), sorted(current)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default=".claudex/upstream-manifest.json")
    parser.add_argument("--ref", help="upstream ref or commit to inspect")
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--check-manifest", action="store_true")
    args = parser.parse_args()

    root = Path.cwd()
    manifest_path = root / args.manifest
    manifest = load_manifest(manifest_path)
    tracked = run(root, "ls-files").splitlines()
    ownership, uncovered = classify(manifest, tracked)
    if uncovered:
        raise RuntimeError("manifest does not classify tracked paths: " + ", ".join(uncovered))

    if args.check_manifest:
        print(f"Manifest covers {len(ownership)} tracked paths.")
        return 0

    upstream = manifest["upstream"]
    repository = upstream["repository"]
    target = args.ref or upstream["base_commit"]
    if args.apply and run(root, "status", "--porcelain"):
        raise RuntimeError("working tree must be clean before applying upstream changes")

    report = {
        "repository": repository,
        "base_commit": upstream["base_commit"],
        "target": target,
        "changes": [],
        "unclassified": uncovered,
    }

    with tempfile.TemporaryDirectory(prefix="claudex-upstream-") as temporary:
        upstream_root = Path(temporary) / "repo"
        subprocess.run(
            ["git", "clone", "--no-tags", "--filter=blob:none", repository, str(upstream_root)],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        run(upstream_root, "checkout", "--detach", target)
        base_root = Path(temporary) / "base"
        run(upstream_root, "worktree", "add", "--detach", str(base_root), upstream["base_commit"])
        paths, _ = workspace_paths(root, upstream_root)
        all_ownership, incoming_uncovered = classify(manifest, paths)
        if incoming_uncovered:
            raise RuntimeError(
                "manifest does not classify current or incoming paths: "
                + ", ".join(incoming_uncovered)
            )
        for path in paths:
            if path not in all_ownership:
                continue
            owner = all_ownership[path]
            current = root / path
            base = base_root / path
            incoming = upstream_root / path
            current_digest = digest(current)
            base_digest = digest(base)
            incoming_digest = digest(incoming)
            findings = classify_change(path)
            if owner == "upstream":
                if current_digest == incoming_digest:
                    continue
                action = "delete" if incoming_digest is None else "update"
                if current_digest != base_digest and incoming_digest != base_digest:
                    action = "conflict"
                    findings.append("local drift from recorded base")
                change = {
                    "path": path,
                    "owner": owner,
                    "action": action,
                    "findings": sorted(set(findings)),
                }
                report["changes"].append(change)
                continue
            if incoming_digest == base_digest:
                continue
            findings.append("upstream changed a Claudex-owned path" if owner == "claudex" else "upstream changed a patched path")
            report["changes"].append(
                {
                    "path": path,
                    "owner": owner,
                    "action": "conflict",
                    "findings": sorted(set(findings)),
                }
            )

        report["changes"].sort(key=lambda item: (item["path"], item["owner"], item["action"]))
        conflicts = [change["path"] for change in report["changes"] if change["action"] == "conflict"]
        if args.apply and conflicts:
            raise RuntimeError("upstream sync has unresolved conflicts: " + ", ".join(conflicts))
        if args.apply:
            for change in report["changes"]:
                if change["owner"] != "upstream":
                    continue
                current = root / change["path"]
                incoming = upstream_root / change["path"]
                if change["action"] == "delete":
                    current.unlink(missing_ok=True)
                else:
                    current.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(incoming, current)

    if args.apply and report["changes"]:
        print("Applied upstream-owned paths. Review, test, and record the new base commit manually.")
    if args.as_json:
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        print(f"Upstream: {report['repository']}")
        print(f"Base: {report['base_commit']}")
        print(f"Target: {report['target']}")
        if not report["changes"]:
            print("No upstream-owned changes.")
        for change in report["changes"]:
            findings = ", ".join(change["findings"]) or "none"
            print(f"{change['action']} {change['path']} ({findings})")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f"claudex-upstream-sync: {error}", file=sys.stderr)
        raise SystemExit(1)
