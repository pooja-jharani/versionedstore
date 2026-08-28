package store

import (
	"sort"
	"sync"
)

// VersionEntry is one entry in a key's version list.
type VersionEntry struct {
	VersionID string
	Timestamp int64
	Offset    int64
}

// Index is the in-memory key -> [(version_id, timestamp, byte_offset), ...]
// structure. It's rebuilt from the log on every boot, so it never touches
// disk itself. Entries per key are kept in ascending timestamp order,
// which lets time-travel lookups use binary search.
type Index struct {
	mu   sync.RWMutex
	data map[string][]VersionEntry
}

func NewIndex() *Index {
	return &Index{data: make(map[string][]VersionEntry)}
}

func (idx *Index) Add(key string, entry VersionEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.data[key] = append(idx.data[key], entry)
}

// Latest returns the most recent version entry for key.
func (idx *Index) Latest(key string) (VersionEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	versions := idx.data[key]
	if len(versions) == 0 {
		return VersionEntry{}, false
	}
	return versions[len(versions)-1], true
}

// At returns the version of key that was current at timestamp ts: the
// latest version with Timestamp <= ts (time-travel query).
func (idx *Index) At(key string, ts int64) (VersionEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	versions := idx.data[key]
	if len(versions) == 0 {
		return VersionEntry{}, false
	}
	i := sort.Search(len(versions), func(i int) bool {
		return versions[i].Timestamp > ts
	})
	if i == 0 {
		return VersionEntry{}, false
	}
	return versions[i-1], true
}

// History returns every version of key, oldest first.
func (idx *Index) History(key string) []VersionEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	versions := idx.data[key]
	out := make([]VersionEntry, len(versions))
	copy(out, versions)
	return out
}

// Keys returns every key currently tracked, sorted.
func (idx *Index) Keys() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	keys := make([]string, 0, len(idx.data))
	for k := range idx.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
