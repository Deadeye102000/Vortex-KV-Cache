package resp

import (
	"bytes"
	"testing"
)

// repeatingReader wraps a buffer and resets it endlessly to avoid giant allocations.
type repeatingReader struct {
	raw *bytes.Reader
}

func (rr *repeatingReader) Read(p []byte) (int, error) {
	n, err := rr.raw.Read(p)
	if err != nil && err.Error() == "EOF" {
		rr.raw.Seek(0, 0)
		return rr.raw.Read(p)
	}
	return n, err
}

func BenchmarkReader(b *testing.B) {
	raw := []byte("*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n")
	rr := &repeatingReader{raw: bytes.NewReader(raw)}
	r := NewReader(rr)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := r.ReadValue()
		if err != nil {
			b.Fatal(err)
		}
	}
}
