# Add end-to-end Claude Code and Desktop compatibility tests

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: test/20260729-e2e-compatibility

## Summary

Add protocol-level end-to-end tests for the exact Anthropic surface Claudex claims to support, including streaming, tools, token counting, model aliases, effort normalization, authentication, and route rejection.

## Problem

Existing focused unit tests protect configuration and model policy, but the most important behavior crosses route registration, Anthropic request parsing, translation, Codex execution, streaming, and response conversion. Regressions can compile and pass package tests while breaking Claude Code or Claude Desktop.

## Agent Prompt

Build a deterministic E2E test harness around the Claudex server with a fake Codex upstream.

1. Start the real Claudex HTTP stack on loopback with temporary config and credentials.
2. Replace only the upstream network boundary with a scripted fake server or transport.
3. Cover `/v1/models`, `/v1/messages`, `/v1/messages/count_tokens`, authentication failures, unsupported routes, and unsupported models.
4. Cover non-streaming and streaming responses, tool use/tool result round trips, system prompts, image rejection, client disconnect, upstream errors, and malformed upstream frames.
5. Assert `max` to `xhigh` normalization and all supported aliases.
6. Store compact golden fixtures for protocol shapes, avoiding secrets and timestamps.
7. Add the suite to CI on Linux and run portable subsets on macOS and Windows.

Do not call real OpenAI or Anthropic services in tests.

## Acceptance Criteria

- [ ] Tests exercise the production route and translation stack rather than calling isolated helpers.
- [ ] All three advertised Claude profiles and direct GPT-5.6 model IDs are covered.
- [ ] Streaming preserves event ordering and terminates correctly on success, error, and cancellation.
- [ ] Unsupported routes return Anthropic-compatible 404 responses.
- [ ] Invalid local authentication is rejected before upstream execution.
- [ ] No test requires external credentials or network access.
- [ ] CI runs the suite reliably without timing-based flakes.

## Test Plan

- Run the new E2E package repeatedly with `-count=20`.
- Run `go test -race` for the server and streaming paths.
- Run `go test ./...` on all supported CI platforms.

## Risks

- Golden fixtures can become brittle if they include irrelevant fields.
- A fake upstream that bypasses too much production code will give false confidence.
