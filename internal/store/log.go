package store

import (
	"bufio"
	"io"
	"os"
	"sync"
)

// Log is an append-only, file-backed write-ahead log. All writes go
// through a native sync.RWMutex (replacing a file-locking library) and
// are fsynced before Append returns, so a completed Append is durable.
// Reads (ReadAll/ReadAt) take the read side of the same lock, so a read
// can never observe a file mid-Append or mid-Truncate.
type Log struct {
	path string
	mu   sync.RWMutex
	file *os.File
}

func OpenLog(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &Log{path: path, file: f}, nil
}

// Append writes a record to the end of the log and fsyncs it, returning
// the byte offset at which the record's frame begins.
func (l *Log) Append(r *Record) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	offset, err := l.file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	data := r.Encode()
	if _, err := l.file.Write(data); err != nil {
		return 0, err
	}
	if err := l.file.Sync(); err != nil {
		return 0, err
	}
	return offset, nil
}

// ReadAll replays the entire log from the start. It returns every valid
// record, the byte offset each one started at, and validEnd — the byte
// position right after the last successfully parsed record.
//
// Replay stops (without returning an error) at the first sign of a torn
// or corrupt tail write, since that's exactly what a crash mid-append
// looks like on disk: everything before validEnd is durable, everything
// from validEnd onward is discarded by the caller.
func (l *Log) ReadAll() ([]*Record, []int64, int64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	f, err := os.Open(l.path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var records []*Record
	var offsets []int64
	var offset int64

	for {
		rec, n, err := DecodeRecord(reader)
		if err == io.EOF {
			break
		}
		if err == ErrShortRead || err == ErrCorruptRecord {
			break
		}
		if err != nil {
			return nil, nil, 0, err
		}
		records = append(records, rec)
		offsets = append(offsets, offset)
		offset += int64(n)
	}

	return records, offsets, offset, nil
}

// ReadAt reads and decodes a single record starting at the given offset.
func (l *Log) ReadAt(offset int64) (*Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	rec, _, err := DecodeRecord(f)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// Truncate cuts the log down to size bytes — used on startup to drop a
// torn tail left by a crash, so future appends land cleanly.
func (l *Log) Truncate(size int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Truncate(size)
}

func (l *Log) Close() error {
	return l.file.Close()
}
