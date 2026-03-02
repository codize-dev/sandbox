package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// makeActionEntry builds a 175-byte Go cache action entry.
func makeActionEntry(actionHex, outputHex string, size, tsNano int64) []byte {
	return []byte(fmt.Sprintf("v1 %s %s %20d %20d\n", actionHex, outputHex, size, tsNano))
}

func TestHandleGet_Hit(t *testing.T) {
	actionID, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	outputHex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	actionHex := hex.EncodeToString(actionID)

	cacheDir := t.TempDir()

	// Create action entry file.
	actionDir := filepath.Join(cacheDir, actionHex[:2])
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionDir, actionHex+"-a"), makeActionEntry(actionHex, outputHex, 42, 1700000000000000000), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create data file.
	dataDir := filepath.Join(cacheDir, outputHex[:2])
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, outputHex+"-d"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := &Request{ID: 1, ActionID: actionID}
	resp := handleGet(cacheDir, req)

	if resp.Miss {
		t.Fatal("expected hit, got miss")
	}
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}
	if hex.EncodeToString(resp.OutputID) != outputHex {
		t.Errorf("unexpected OutputID: %s", hex.EncodeToString(resp.OutputID))
	}
	if resp.Size != 42 {
		t.Errorf("expected Size 42, got %d", resp.Size)
	}
	if resp.DiskPath == "" {
		t.Error("expected DiskPath to be set")
	}
}

func TestHandleGet_MissNoActionFile(t *testing.T) {
	actionID, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	cacheDir := t.TempDir()

	req := &Request{ID: 2, ActionID: actionID}
	resp := handleGet(cacheDir, req)

	if !resp.Miss {
		t.Fatal("expected miss")
	}
}

func TestHandleGet_MissInvalidActionIDLength(t *testing.T) {
	cacheDir := t.TempDir()

	req := &Request{ID: 3, ActionID: []byte("short")}
	resp := handleGet(cacheDir, req)

	if !resp.Miss {
		t.Fatal("expected miss for invalid ActionID length")
	}
}

func TestHandleGet_MissNoDataFile(t *testing.T) {
	actionID, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	outputHex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	actionHex := hex.EncodeToString(actionID)

	cacheDir := t.TempDir()

	// Create action entry file but NOT the data file.
	actionDir := filepath.Join(cacheDir, actionHex[:2])
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionDir, actionHex+"-a"), makeActionEntry(actionHex, outputHex, 42, 1700000000000000000), 0o644); err != nil {
		t.Fatal(err)
	}

	req := &Request{ID: 4, ActionID: actionID}
	resp := handleGet(cacheDir, req)

	if !resp.Miss {
		t.Fatal("expected miss when data file is missing")
	}
}

func TestHandleGet_MissInvalidFormat(t *testing.T) {
	actionID, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	actionHex := hex.EncodeToString(actionID)

	cacheDir := t.TempDir()

	// Create action entry file with invalid content.
	actionDir := filepath.Join(cacheDir, actionHex[:2])
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionDir, actionHex+"-a"), []byte("invalid content"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := &Request{ID: 5, ActionID: actionID}
	resp := handleGet(cacheDir, req)

	if !resp.Miss {
		t.Fatal("expected miss for invalid format")
	}
}
