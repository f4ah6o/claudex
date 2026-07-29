# Establish release, platform, and security CI gates

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: ci/20260729-release-security-gates

## Summary

Create a complete CI and release contract for the supported CLI across Linux, macOS, and Windows and for Desktop artifacts on macOS and Linux, with reproducible metadata, vulnerability checks, secret scanning, checksums, and smoke tests.

## Problem

The development documentation lists local `go test`, build, `govulncheck`, and `gitleaks` commands, but users depend on source builds and platform launchers without a clearly enforced release pipeline. The Dockerfile also builds with CGO and floating base image tags, which weakens reproducibility and increases the runtime supply-chain surface.

## Agent Prompt

Implement CI and release automation appropriate to the supported product surface.

1. Run formatting, vet/static analysis, focused tests, full tests, race tests where supported, `govulncheck`, and secret scanning.
2. Build `cmd/claudex` for supported OS/architecture pairs and build platform launchers where they are actually supported.
3. Add smoke tests that start the binary with a temporary valid config, verify loopback binding and allowed routes, then terminate cleanly.
4. Produce tagged release archives with version/commit/build-date metadata, SHA-256 checksums, SBOMs, and provenance/attestations supported by GitHub Actions.
5. Pin GitHub Actions and container base images by immutable versions or digests with a documented update process.
6. Review whether CGO is required. Prefer `CGO_ENABLED=0` and a non-root minimal runtime image when compatible; otherwise document and test the requirement.
7. Ensure release jobs use least-privilege permissions and trusted publishing mechanisms.
8. Keep release artifacts separate for the gateway CLI and Desktop launcher to avoid ambiguous installation instructions.

Do not publish artifacts from pull-request workflows or expose credentials to untrusted code.

## Acceptance Criteria

- [ ] Required PR checks cover format, static analysis, tests, policy/E2E checks, vulnerabilities, and secrets.
- [ ] Supported platform artifacts are built and smoke-tested in CI.
- [ ] Tagged releases include checksums, SBOM, provenance, and embedded version metadata.
- [ ] Workflow actions and container images are immutably pinned.
- [ ] Release permissions are minimal and no long-lived publishing secret is required where trusted publishing is available.
- [ ] Docker runs as non-root, contains no build toolchain, and remains loopback-only by default.
- [ ] README installation instructions match the artifacts actually produced.

## Test Plan

- Validate workflows with pull-request and tag fixtures.
- Inspect binaries with `claudex version` and verify archive/checksum consistency.
- Run container smoke tests and assert non-root UID, listener address, route policy, and clean shutdown.
- Verify forked pull requests cannot access release credentials.

## Risks

- macOS application signing and notarization require separately managed credentials and should be isolated from ordinary CI.
- Race tests and cross-platform Desktop tests may require platform-specific exclusions with explicit justification.
