package resp

import (
	"bytes"
	"io"
	"testing"
)

func TestRESP_SimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteSimpleString("OK"); err != nil {
		t.Fatalf("failed to write simple string: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read simple string: %v", err)
	}

	if val.Type != TypeSimpleString || val.Str != "OK" {
		t.Fatalf("unexpected value: %+v", val)
	}
}

func TestRESP_Error(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteError("ERR unknown command"); err != nil {
		t.Fatalf("failed to write error: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}

	if val.Type != TypeError || val.Str != "ERR unknown command" {
		t.Fatalf("unexpected error value: %+v", val)
	}
}

func TestRESP_Integer(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteInteger(1001); err != nil {
		t.Fatalf("failed to write integer: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read integer: %v", err)
	}

	if val.Type != TypeInteger || val.Num != 1001 {
		t.Fatalf("unexpected integer value: %+v", val)
	}
}

func TestRESP_BulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	data := []byte("vortex_cache_data")
	if err := w.WriteBulkString(data); err != nil {
		t.Fatalf("failed to write bulk string: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read bulk string: %v", err)
	}

	if val.Type != TypeBulkString || !bytes.Equal(val.Bulk, data) {
		t.Fatalf("unexpected bulk string value: %+v", val)
	}
}

func TestRESP_NullBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteNull(); err != nil {
		t.Fatalf("failed to write null bulk string: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read null bulk string: %v", err)
	}

	if val.Type != TypeBulkString || !val.Nil {
		t.Fatalf("expected null bulk string, got: %+v", val)
	}
}

func TestRESP_Array(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	arr := []Value{
		NewBulkStringFromString("SET"),
		NewBulkStringFromString("mykey"),
		NewBulkStringFromString("myval"),
	}

	if err := w.WriteArray(arr); err != nil {
		t.Fatalf("failed to write array: %v", err)
	}
	_ = w.Flush()

	r := NewReader(&buf)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to read array: %v", err)
	}

	if val.Type != TypeArray || len(val.Array) != 3 {
		t.Fatalf("unexpected array value: %+v", val)
	}

	if string(val.Array[0].Bulk) != "SET" || string(val.Array[1].Bulk) != "mykey" || string(val.Array[2].Bulk) != "myval" {
		t.Fatalf("unexpected array elements: %+v", val.Array)
	}
}

// slowReader simulates fragmented TCP packet reads over network stream
type slowReader struct {
	data []byte
	pos  int
}

func (s *slowReader) Read(p []byte) (n int, err error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	// Return at most 1 byte per read to test TCP fragmentation safety
	p[0] = s.data[s.pos]
	s.pos++
	return 1, nil
}

func TestRESP_FragmentedTCPRead(t *testing.T) {
	raw := []byte("*2\r\n$3\r\nGET\r\n$5\r\nhello\r\n")
	slow := &slowReader{data: raw}

	r := NewReader(slow)
	val, err := r.ReadValue()
	if err != nil {
		t.Fatalf("failed to parse fragmented TCP stream: %v", err)
	}

	if val.Type != TypeArray || len(val.Array) != 2 {
		t.Fatalf("unexpected array length: %d", len(val.Array))
	}
	if string(val.Array[0].Bulk) != "GET" || string(val.Array[1].Bulk) != "hello" {
		t.Fatalf("unexpected command contents: %+v", val.Array)
	}
}
