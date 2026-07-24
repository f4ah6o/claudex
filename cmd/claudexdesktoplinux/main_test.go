package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyGatewayConfigurationWritesLinuxThirdPartyProfile(t *testing.T) {
	root := t.TempDir()
	profileID := "00000000-0000-4000-8000-000000000001"
	paths := desktopPaths{
		StandardConfig:   filepath.Join(root, "Claude", "claude_desktop_config.json"),
		ThirdPartyConfig: filepath.Join(root, "Claude-3p", "claude_desktop_config.json"),
		LibraryMeta:      filepath.Join(root, "Claude-3p", "configLibrary", "_meta.json"),
		LibraryProfile:   filepath.Join(root, "Claude-3p", "configLibrary", profileID+".json"),
		Backup:           filepath.Join(root, "claudex", backupFileName),
	}

	if err := applyGatewayConfiguration(paths, "http://127.0.0.1:8317/", "local-key"); err != nil {
		t.Fatal(err)
	}

	standard := readTestObject(t, paths.StandardConfig)
	if standard["deploymentMode"] != "3p" {
		t.Fatalf("standard deploymentMode = %#v", standard["deploymentMode"])
	}
	thirdParty := readTestObject(t, paths.ThirdPartyConfig)
	if thirdParty["deploymentMode"] != "3p" {
		t.Fatalf("third-party deploymentMode = %#v", thirdParty["deploymentMode"])
	}

	profile := readTestObject(t, paths.LibraryProfile)
	if profile["inferenceProvider"] != "gateway" {
		t.Fatalf("inferenceProvider = %#v", profile["inferenceProvider"])
	}
	if profile["inferenceGatewayBaseUrl"] != "http://127.0.0.1:8317" {
		t.Fatalf("inferenceGatewayBaseUrl = %#v", profile["inferenceGatewayBaseUrl"])
	}
	models, ok := profile["inferenceModels"].([]any)
	if !ok || len(models) != 3 {
		t.Fatalf("inferenceModels must be a three-entry JSON array, got %#v", profile["inferenceModels"])
	}

	meta := readTestObject(t, paths.LibraryMeta)
	if meta["appliedId"] != profileID {
		t.Fatalf("appliedId = %#v", meta["appliedId"])
	}
	entries, ok := meta["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("entries = %#v", meta["entries"])
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["provider"] != "gateway" {
		t.Fatalf("entry = %#v", entries[0])
	}
}

func readTestObject(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err = json.Unmarshal(contents, &object); err != nil {
		t.Fatal(err)
	}
	return object
}
