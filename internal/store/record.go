package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

// ErrCorruptRecord means a record's stored checksum didn't match its
// contents (bit rot / disk corruption).
var ErrCorruptRecord = errors.New("corrupt record: checksum mismatch")

// ErrShortRead means the stream ended in the middle of a record. This is
// exactly what a crash mid-append looks like on disk, so callers treat it
// as "stop replaying here", not as a fatal error.
var ErrShortRead = errors.New("incomplete record")

// Record is one versioned write: {key, value, timestamp, version_id, checksum}.
type Record struct {
	Key       string
	Value     []byte
	Timestamp int64  // unix nanoseconds
	VersionID string // 36-char UUID v4 string
	Checksum  uint32
}

func computeChecksum(key string, value []byte, ts int64, versionID string) uint32 {
	h := crc32.NewIEEE()
	h.Write([]byte(key))
	h.Write(value)
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(ts))
	h.Write(tsBytes)
	h.Write([]byte(versionID))
	return h.Sum32()
}

// Encode serializes the record into a length-prefixed binary frame:
//
//	[4B total payload length]
//	[2B keyLen][key]
//	[4B valueLen][value]
//	[8B timestamp]
//	[36B versionID]
//	[4B crc32 checksum]
//
// The outer length prefix is what lets replay detect a torn tail write:
// if fewer bytes follow than promised, the process crashed mid-append.
func (r *Record) Encode() []byte {
	payload := new(bytes.Buffer)

	keyBytes := []byte(r.Key)
	kl := make([]byte, 2)
	binary.BigEndian.PutUint16(kl, uint16(len(keyBytes)))
	payload.Write(kl)
	payload.Write(keyBytes)

	vl := make([]byte, 4)
	binary.BigEndian.PutUint32(vl, uint32(len(r.Value)))
	payload.Write(vl)
	payload.Write(r.Value)

	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(r.Timestamp))
	payload.Write(tsBytes)

	vidBytes := []byte(r.VersionID)
	if len(vidBytes) != 36 {
		panic("versionID must be a 36-character UUID string")
	}
	payload.Write(vidBytes)

	checksum := computeChecksum(r.Key, r.Value, r.Timestamp, r.VersionID)
	cs := make([]byte, 4)
	binary.BigEndian.PutUint32(cs, checksum)
	payload.Write(cs)

	frame := new(bytes.Buffer)
	total := make([]byte, 4)
	binary.BigEndian.PutUint32(total, uint32(payload.Len()))
	frame.Write(total)
	frame.Write(payload.Bytes())

	return frame.Bytes()
}

// DecodeRecord reads one length-prefixed record from r.
// It returns (nil, 0, io.EOF) at a clean stream end.
// It returns (nil, n, ErrShortRead) when the stream ends mid-record.
// It returns (nil, n, ErrCorruptRecord) when the checksum doesn't match.
// On success it returns (record, bytesConsumed, nil).
func DecodeRecord(r io.Reader) (*Record, int, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		if err == io.EOF {
			return nil, 0, io.EOF
		}
		return nil, 0, ErrShortRead
	}
	total := binary.BigEndian.Uint32(lenBuf)

	payload := make([]byte, total)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, ErrShortRead
	}
	consumed := 4 + int(total)

	buf := bytes.NewReader(payload)

	kl := make([]byte, 2)
	if _, err := io.ReadFull(buf, kl); err != nil {
		return nil, consumed, ErrShortRead
	}
	keyLen := binary.BigEndian.Uint16(kl)
	keyBytes := make([]byte, keyLen)
	if _, err := io.ReadFull(buf, keyBytes); err != nil {
		return nil, consumed, ErrShortRead
	}

	vl := make([]byte, 4)
	if _, err := io.ReadFull(buf, vl); err != nil {
		return nil, consumed, ErrShortRead
	}
	valueLen := binary.BigEndian.Uint32(vl)
	value := make([]byte, valueLen)
	if _, err := io.ReadFull(buf, value); err != nil {
		return nil, consumed, ErrShortRead
	}

	tsBytes := make([]byte, 8)
	if _, err := io.ReadFull(buf, tsBytes); err != nil {
		return nil, consumed, ErrShortRead
	}
	ts := int64(binary.BigEndian.Uint64(tsBytes))

	vidBytes := make([]byte, 36)
	if _, err := io.ReadFull(buf, vidBytes); err != nil {
		return nil, consumed, ErrShortRead
	}
	versionID := string(vidBytes)

	csBytes := make([]byte, 4)
	if _, err := io.ReadFull(buf, csBytes); err != nil {
		return nil, consumed, ErrShortRead
	}
	checksum := binary.BigEndian.Uint32(csBytes)

	rec := &Record{
		Key:       string(keyBytes),
		Value:     value,
		Timestamp: ts,
		VersionID: versionID,
		Checksum:  checksum,
	}

	expected := computeChecksum(rec.Key, rec.Value, rec.Timestamp, rec.VersionID)
	if expected != checksum {
		return nil, consumed, ErrCorruptRecord
	}

	return rec, consumed, nil
}
