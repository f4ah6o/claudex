# Minimize and strictly validate Claudex configuration

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: fix/20260729-config-contract

## Summary

Replace inheritance of the broad upstream configuration surface with a small Claudex-owned configuration contract that rejects unknown, unsafe, and ineffective settings.

## Problem

The example configuration still includes disabled management, plugin, routing, quota, and generic proxy settings. Runtime policy rejects unsafe combinations, but accepting a broad upstream schema creates ambiguity: unsupported keys may appear valid, upstream defaults can change, and users cannot tell which settings Claudex actually honors.

## Agent Prompt

Create a Claudex-specific configuration type and decoding boundary.

1. Inventory every configuration value read by the Claudex binary.
2. Define a minimal public schema containing only listener, local client authentication, Codex authentication location, retry behavior, logging, supported aliases, and explicitly supported runtime options.
3. Decode with unknown-field rejection and clear field-path errors.
4. Convert the minimal type into internal upstream types only after validation.
5. Reject non-loopback hosts, placeholder or empty client keys, unsupported models, duplicate aliases, insecure file modes, and conflicting environment/flag values.
6. Generate initial configuration with a cryptographically random local key and restrictive permissions.
7. Provide a migration error for legacy broad configurations; do not silently ignore unsupported fields.
8. Update English/Japanese documentation and launcher bootstrap behavior.

Do not expose management, plugin, other-provider, hosted, or multi-user settings.

## Acceptance Criteria

- [ ] The accepted configuration surface is documented and represented by one Claudex-owned type.
- [ ] Unknown YAML fields fail startup with actionable errors.
- [ ] Generated configuration contains no placeholder credential and uses owner-only permissions where supported.
- [ ] Environment variables and flags have documented precedence and conflict tests.
- [ ] Aliases can target only `gpt-5.6` or `gpt-5.6-*` and cannot reintroduce Haiku.
- [ ] Legacy unsupported keys produce a migration message rather than being ignored.
- [ ] Secrets are redacted from logs and validation errors.

## Test Plan

- Add table-driven decode and validation tests for every field.
- Test unknown keys, duplicate YAML keys, malformed durations, weak permissions, symlinks, and conflicting overrides.
- Run bootstrap tests on Unix and Windows-compatible permission paths.
- Run `go test ./internal/claudex ./cmd/claudex ./cmd/claudexdesktop` and `go test ./...`.

## Risks

- Strict decoding is a deliberate compatibility break for undocumented upstream keys and requires a clear migration error.
- Windows ACL behavior cannot be represented only by Unix mode bits.
