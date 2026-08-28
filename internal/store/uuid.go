package store

import "crypto/rand"

// generateUUID returns a random UUID v4 string (36 chars), built entirely
// from crypto/rand — no UUID library dependency.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read on a real OS practically never fails
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

func formatUUID(b []byte) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, 36)
	pos := 0
	writeGroup := func(group []byte) {
		for _, c := range group {
			buf[pos] = hexDigits[c>>4]
			buf[pos+1] = hexDigits[c&0x0f]
			pos += 2
		}
	}
	writeGroup(b[0:4])
	buf[pos] = '-'
	pos++
	writeGroup(b[4:6])
	buf[pos] = '-'
	pos++
	writeGroup(b[6:8])
	buf[pos] = '-'
	pos++
	writeGroup(b[8:10])
	buf[pos] = '-'
	pos++
	writeGroup(b[10:16])
	return string(buf)
}
