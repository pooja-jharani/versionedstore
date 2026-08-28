package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestSetGetHistory(t *testing.T) {
	s := tempStore(t)
	defer s.Close()

	v1, err := s.Set("name", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set("name", []byte("bob")); err != nil {
		t.Fatal(err)
	}
	v3, err := s.Set("name", []byte("carol"))
	if err != nil {
		t.Fatal(err)
	}

	val, err := s.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "carol" {
		t.Fatalf("expected carol, got %s", val)
	}

	hist, err := s.History("name")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(hist))
	}
	if hist[0].VersionID != v1 || hist[2].VersionID != v3 {
		t.Fatalf("history not in expected order")
	}
}

func TestGetAt(t *testing.T) {
	s := tempStore(t)
	defer s.Close()

	if _, err := s.Set("k", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	mid := time.Now().UnixNano()
	time.Sleep(2 * time.Millisecond)
	if _, err := s.Set("k", []byte("v2")); err != nil {
		t.Fatal(err)
	}

	val, err := s.GetAt("k", mid)
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "v1" {
		t.Fatalf("expected v1 at mid timestamp, got %s", val)
	}
}

func TestDiffAndRollback(t *testing.T) {
	s := tempStore(t)
	defer s.Close()

	if _, err := s.Set("doc", []byte("line1\nline2\nline3")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set("doc", []byte("line1\nline2-changed\nline3\nline4")); err != nil {
		t.Fatal(err)
	}

	diff, err := s.Diff("doc", "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "- line2") || !strings.Contains(diff, "+ line2-changed") {
		t.Fatalf("unexpected diff output:\n%s", diff)
	}

	newVer, err := s.Rollback("doc", "1")
	if err != nil {
		t.Fatal(err)
	}
	val, err := s.Get("doc")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "line1\nline2\nline3" {
		t.Fatalf("rollback did not restore original value, got %s", val)
	}
	hist, _ := s.History("doc")
	if hist[len(hist)-1].VersionID != newVer {
		t.Fatalf("rollback should append a new version, not rewrite history")
	}
}

// TestCrashRecovery simulates the exact demo moment from the plan:
// write, kill process mid-write, restart, prove data intact.
func TestCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.log")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set("key", []byte("committed-value")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate a crash mid-write: append a frame whose length prefix
	// promises more bytes than actually follow.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	torn := []byte{0x00, 0x00, 0x00, 0x64, 'g', 'a', 'r', 'b', 'a', 'g', 'e'} // claims 100 bytes, supplies 7
	if _, err := f.Write(torn); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: replay must recover the committed record and silently
	// stop at the torn tail instead of erroring out.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after crash should succeed, got: %v", err)
	}

	val, err := s2.Get("key")
	if err != nil {
		t.Fatalf("expected committed value to survive crash, got err: %v", err)
	}
	if string(val) != "committed-value" {
		t.Fatalf("expected committed-value, got %s", val)
	}

	// A write after recovery must land cleanly (the torn tail was
	// truncated on Open, not just skipped over).
	if _, err := s2.Set("key", []byte("after-recovery")); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}

	s3, err := Open(path)
	if err != nil {
		t.Fatalf("second reopen should succeed: %v", err)
	}
	defer s3.Close()

	hist, err := s3.History("key")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 clean versions after truncating torn tail, got %d", len(hist))
	}
	val, err = s3.Get("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "after-recovery" {
		t.Fatalf("expected after-recovery on reopen, got %s", val)
	}
}
