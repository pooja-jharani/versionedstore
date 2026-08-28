package store

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrKeyNotFound = errors.New("key not found")
var ErrVersionNotFound = errors.New("version not found")

// Store is the versioned key-value store: an append-only Log plus an
// in-memory Index rebuilt from it on every Open.
type Store struct {
	log       *Log
	index     *Index
	lastNanos int64 // guarantees strictly increasing timestamps across rapid writes
}

// Open opens (or creates) the log at path, replays it to rebuild the
// index, and truncates away any torn tail left by a crash mid-write.
func Open(path string) (*Store, error) {
	log, err := OpenLog(path)
	if err != nil {
		return nil, err
	}
	idx := NewIndex()

	records, offsets, validEnd, err := log.ReadAll()
	if err != nil {
		return nil, err
	}
	for i, rec := range records {
		idx.Add(rec.Key, VersionEntry{
			VersionID: rec.VersionID,
			Timestamp: rec.Timestamp,
			Offset:    offsets[i],
		})
	}

	if err := log.Truncate(validEnd); err != nil {
		return nil, err
	}

	var last int64
	if len(records) > 0 {
		last = records[len(records)-1].Timestamp
	}

	return &Store{log: log, index: idx, lastNanos: last}, nil
}

func (s *Store) Close() error {
	return s.log.Close()
}

func (s *Store) nextTimestamp() int64 {
	now := time.Now().UnixNano()
	if now <= s.lastNanos {
		now = s.lastNanos + 1
	}
	s.lastNanos = now
	return now
}

// Set appends a new version of key and returns its version ID.
func (s *Store) Set(key string, value []byte) (string, error) {
	ts := s.nextTimestamp()
	versionID := generateUUID()
	rec := &Record{
		Key:       key,
		Value:     value,
		Timestamp: ts,
		VersionID: versionID,
	}
	offset, err := s.log.Append(rec)
	if err != nil {
		return "", err
	}
	s.index.Add(key, VersionEntry{VersionID: versionID, Timestamp: ts, Offset: offset})
	return versionID, nil
}

// Get returns the latest value for key.
func (s *Store) Get(key string) ([]byte, error) {
	entry, ok := s.index.Latest(key)
	if !ok {
		return nil, ErrKeyNotFound
	}
	rec, err := s.log.ReadAt(entry.Offset)
	if err != nil {
		return nil, err
	}
	return rec.Value, nil
}

// GetAt returns the value of key as of timestamp ts (unix nanoseconds) —
// the time-travel query.
func (s *Store) GetAt(key string, ts int64) ([]byte, error) {
	entry, ok := s.index.At(key, ts)
	if !ok {
		return nil, ErrKeyNotFound
	}
	rec, err := s.log.ReadAt(entry.Offset)
	if err != nil {
		return nil, err
	}
	return rec.Value, nil
}

// HistoryEntry is one version as returned by History.
type HistoryEntry struct {
	VersionID string
	Timestamp int64
	Value     []byte
}

// History returns every version of key, oldest first.
func (s *Store) History(key string) ([]HistoryEntry, error) {
	versions := s.index.History(key)
	if len(versions) == 0 {
		return nil, ErrKeyNotFound
	}
	out := make([]HistoryEntry, 0, len(versions))
	for _, v := range versions {
		rec, err := s.log.ReadAt(v.Offset)
		if err != nil {
			return nil, err
		}
		out = append(out, HistoryEntry{VersionID: v.VersionID, Timestamp: v.Timestamp, Value: rec.Value})
	}
	return out, nil
}

// resolveVersion finds a value for key at a given version, accepting
// either a full version ID or the 1-based position shown by History
// (1 = oldest) — makes the CLI usable without copy-pasting UUIDs.
func (s *Store) resolveVersion(key, version string) ([]byte, error) {
	versions := s.index.History(key)
	if len(versions) == 0 {
		return nil, ErrKeyNotFound
	}
	if n, err := strconv.Atoi(version); err == nil && n >= 1 && n <= len(versions) {
		rec, err := s.log.ReadAt(versions[n-1].Offset)
		if err != nil {
			return nil, err
		}
		return rec.Value, nil
	}
	for _, v := range versions {
		if v.VersionID == version {
			rec, err := s.log.ReadAt(v.Offset)
			if err != nil {
				return nil, err
			}
			return rec.Value, nil
		}
	}
	return nil, ErrVersionNotFound
}

// Diff returns a line-based diff between two versions of key.
func (s *Store) Diff(key, v1, v2 string) (string, error) {
	val1, err := s.resolveVersion(key, v1)
	if err != nil {
		return "", fmt.Errorf("version %s: %w", v1, err)
	}
	val2, err := s.resolveVersion(key, v2)
	if err != nil {
		return "", fmt.Errorf("version %s: %w", v2, err)
	}
	return LineDiff(string(val1), string(val2)), nil
}

// Rollback writes a new version of key equal to the value at `version`
// and returns the new version ID. Rollback is itself just a new write —
// history is never rewritten, only appended to (same as the log's
// append-only guarantee everywhere else).
func (s *Store) Rollback(key, version string) (string, error) {
	val, err := s.resolveVersion(key, version)
	if err != nil {
		return "", err
	}
	return s.Set(key, val)
}
