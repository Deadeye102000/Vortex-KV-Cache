package resp

import (
	"bytes"
	"testing"
)

func BenchmarkWriterInteger(b *testing.B) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w.WriteInteger(123456789)
		buf.Reset()
	}
}

func BenchmarkWriterBulk(b *testing.B) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	data := make([]byte, 1024)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w.WriteBulkString(data)
		buf.Reset()
	}
}

func BenchmarkWriterArray(b *testing.B) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	arr := make([]Value, 15)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w.WriteArray(arr)
		buf.Reset()
	}
}
