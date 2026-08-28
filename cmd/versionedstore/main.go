package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"versionedstore/internal/store"
)

const dataFile = "data/versionedstore.log"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if err := os.MkdirAll("data", 0755); err != nil {
		fatal(err)
	}

	s, err := store.Open(dataFile)
	if err != nil {
		fatal(err)
	}
	defer s.Close()

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "set":
		cmdSet(s, args)
	case "get":
		cmdGet(s, args)
	case "history":
		cmdHistory(s, args)
	case "diff":
		cmdDiff(s, args)
	case "rollback":
		cmdRollback(s, args)
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdSet(s *store.Store, args []string) {
	if len(args) < 2 {
		fatal(fmt.Errorf("usage: versionedstore set <key> <value>"))
	}
	versionID, err := s.Set(args[0], []byte(args[1]))
	if err != nil {
		fatal(err)
	}
	fmt.Printf("OK version=%s\n", versionID)
}

func cmdGet(s *store.Store, args []string) {
	if len(args) < 1 {
		fatal(fmt.Errorf("usage: versionedstore get <key> [--at <unix-seconds|RFC3339>]"))
	}
	key := args[0]

	if len(args) >= 3 && args[1] == "--at" {
		ts, err := parseTimestamp(args[2])
		if err != nil {
			fatal(err)
		}
		val, err := s.GetAt(key, ts)
		if err != nil {
			fatal(err)
		}
		fmt.Println(string(val))
		return
	}

	val, err := s.Get(key)
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(val))
}

func cmdHistory(s *store.Store, args []string) {
	if len(args) < 1 {
		fatal(fmt.Errorf("usage: versionedstore history <key>"))
	}
	entries, err := s.History(args[0])
	if err != nil {
		fatal(err)
	}
	for i, e := range entries {
		t := time.Unix(0, e.Timestamp).Format(time.RFC3339Nano)
		fmt.Printf("%d) version=%s time=%s value=%q\n", i+1, e.VersionID, t, string(e.Value))
	}
}

func cmdDiff(s *store.Store, args []string) {
	if len(args) < 3 {
		fatal(fmt.Errorf("usage: versionedstore diff <key> <v1> <v2>"))
	}
	out, err := s.Diff(args[0], args[1], args[2])
	if err != nil {
		fatal(err)
	}
	fmt.Print(out)
}

func cmdRollback(s *store.Store, args []string) {
	if len(args) < 2 {
		fatal(fmt.Errorf("usage: versionedstore rollback <key> <version>"))
	}
	newVersion, err := s.Rollback(args[0], args[1])
	if err != nil {
		fatal(err)
	}
	fmt.Printf("OK rolled back, new version=%s\n", newVersion)
}

// parseTimestamp accepts either unix seconds ("1735000000") or RFC3339
// ("2026-08-28T10:00:00Z") and returns unix nanoseconds.
func parseTimestamp(raw string) (int64, error) {
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return sec * int64(time.Second), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, fmt.Errorf("could not parse timestamp %q (use unix seconds or RFC3339)", raw)
	}
	return t.UnixNano(), nil
}

func printUsage() {
	fmt.Println(`VersionedStore — append-only versioned key-value store

Usage:
  versionedstore set <key> <value>
  versionedstore get <key> [--at <unix-seconds|RFC3339>]
  versionedstore history <key>
  versionedstore diff <key> <v1> <v2>
  versionedstore rollback <key> <version>

<v1>/<v2>/<version> above accept either a full version ID or the
1-based position shown by 'history' (1 = oldest).`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
