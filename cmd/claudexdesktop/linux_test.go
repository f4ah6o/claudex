//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestLinuxDesktopEnvironmentReplacesExistingValues(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 8317}
	environment := linuxDesktopEnvironment([]string{
		"PATH=/usr/bin",
		linuxGatewayAPIKeyEnv + "=old-key",
		linuxDesktopModeEnv + "=0",
	}, cfg, "new-key")

	values := make(map[string][]string)
	for _, item := range environment {
		parts := strings.SplitN(item, "=", 2)
		values[parts[0]] = append(values[parts[0]], parts[1])
	}
	if got := values[linuxGatewayAPIKeyEnv]; len(got) != 1 || got[0] != "new-key" {
		t.Fatalf("gateway key environment = %#v", got)
	}
	if got := values[linuxDesktopModeEnv]; len(got) != 1 || got[0] != "1" {
		t.Fatalf("desktop mode environment = %#v", got)
	}
	if got := values[linuxGatewayBaseURLEnv]; len(got) != 1 || got[0] != "http://127.0.0.1:8317" {
		t.Fatalf("gateway URL environment = %#v", got)
	}
	if got := values[linuxInferenceModelsEnv]; len(got) != 1 || got[0] != claudex.InferenceModelsValue() {
		t.Fatalf("inference models environment = %#v", got)
	}
}

func TestLinuxDesktopCommandAndProcessOverrides(t *testing.T) {
	t.Setenv(linuxDesktopCommandEnv, "/opt/claude-desktop/claudex/claudex-desktop")
	t.Setenv(linuxDesktopProcessEnv, "custom-desktop")
	if got := linuxDesktopCommand(); got != "/opt/claude-desktop/claudex/claudex-desktop" {
		t.Fatalf("desktop command = %q", got)
	}
	if got := linuxDesktopProcessName(); got != "custom-desktop" {
		t.Fatalf("desktop process name = %q", got)
	}
}

func TestLinuxDesktopEnvironmentDoesNotUseProcessArguments(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 8317}
	environment := linuxDesktopEnvironment(os.Environ(), cfg, "secret-key")
	for _, item := range environment {
		if strings.Contains(item, "--api-key") || strings.Contains(item, "--gateway") {
			t.Fatalf("gateway settings unexpectedly look like command arguments: %q", item)
		}
	}
}

func TestLinuxConfirmationDialogTreatsCancelAsUnconfirmed(t *testing.T) {
	dialog := filepath.Join(t.TempDir(), "dialog")
	if err := os.WriteFile(dialog, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	confirmed, err := runLinuxConfirmationDialog(dialog)
	if err != nil {
		t.Fatalf("runLinuxConfirmationDialog() error = %v", err)
	}
	if confirmed {
		t.Fatal("runLinuxConfirmationDialog() confirmed a cancelled dialog")
	}
}
