package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopSessionLockAllowsOnlyOneOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-session.lock")
	first, err := acquireDesktopSession(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.release()
	if _, err = acquireDesktopSession(path); err == nil {
		t.Fatal("second desktop session acquired the lock")
	}
}

func TestDesktopSessionLockReclaimsDeadOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-session.lock")
	contents, err := json.Marshal(desktopSessionLock{
		PID:          999999,
		Executable:   "/does/not/exist",
		ProcessStart: "dead",
		Transaction:  "stale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	owner, err := acquireDesktopSession(path)
	if err != nil {
		t.Fatal(err)
	}
	owner.release()
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session lock still exists: %v", err)
	}
}

func TestPreferenceValueChecksumChangesWhenValueChanges(t *testing.T) {
	first := newPreferenceValue(true, "one")
	second := newPreferenceValue(true, "two")
	if first.Checksum == "" || first == second {
		t.Fatal("preference checksum did not identify changed value")
	}
}
