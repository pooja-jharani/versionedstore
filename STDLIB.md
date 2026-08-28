# STDLIB.md — Zero-Dependency Substitutions

`go.mod` has no `require` block. Every capability below is built on the
Go standard library only.

| Normally Used                     | Replaced With (stdlib)                                              | Where |
|------------------------------------|-----------------------------------------------------------------------|-------|
| leveldb / rocksdb                 | Manual offset-indexed in-memory map                                   | `internal/store/index.go` |
| git (for history tracking)        | Custom append-only WAL, one frame per write                           | `internal/store/log.go` |
| sqlite (history table)            | `map[string][]VersionEntry` index, rebuilt from the log on boot       | `internal/store/index.go` |
| File locking libraries            | `sync.Mutex` guarding every log append/truncate                       | `internal/store/log.go` |
| msgpack / protobuf                | Custom length-prefixed binary record format                           | `internal/store/record.go` |
| UUID libraries                    | UUID v4 built from `crypto/rand`, formatted by hand                   | `internal/store/uuid.go` |
| Checksum libraries                | `hash/crc32` (IEEE) over key+value+timestamp+versionID                | `internal/store/record.go` |
| CLI arg parser libs (cobra/click) | Manual `os.Args` switch/case dispatch                                 | `cmd/versionedstore/main.go` |
| Diff libraries (go-diff, etc.)    | Custom LCS (longest common subsequence) line diff, written from scratch | `internal/store/diff.go` |
| Logging frameworks                | `fmt.Println` / `fmt.Fprintln(os.Stderr, ...)`                        | `cmd/versionedstore/main.go` |
| Binary search / index libs        | `sort.Search` over each key's version slice for time-travel lookups   | `internal/store/index.go` |
| watchdog / fsnotify               | Not needed — each CLI invocation opens the store, replays the log, runs one command, and exits. No long-running file watch is required by this design. |

**12 substitutions documented** (target: 10+).

## Dependency proof

```
$ cat go.mod
module versionedstore

go 1.22
```

No `require` block — nothing to fetch, nothing to vendor.
