package resp

import (
	"bufio"
	"io"
	"strconv"
)

var (
	crlf = []byte("\r\n")
)

// Writer encodes and writes RESP2 protocol frames.
type Writer struct {
	writer *bufio.Writer
}

// NewWriter wraps an io.Writer (such as a net.Conn) in a buffered RESP writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
	}
}

// WriteValue serializes and writes a full Value frame.
func (w *Writer) WriteValue(v Value) error {
	switch v.Type {
	case TypeSimpleString:
		return w.WriteSimpleString(v.Str)
	case TypeError:
		return w.WriteError(v.Str)
	case TypeInteger:
		return w.WriteInteger(v.Num)
	case TypeBulkString:
		if v.Nil {
			return w.WriteNull()
		}
		return w.WriteBulkString(v.Bulk)
	case TypeArray:
		if v.Nil {
			return w.WriteNullArray()
		}
		return w.WriteArray(v.Array)
	default:
		return w.WriteError("ERR unknown RESP type")
	}
}

// WriteSimpleString writes +string\r\n.
func (w *Writer) WriteSimpleString(s string) error {
	if err := w.writer.WriteByte(byte(TypeSimpleString)); err != nil {
		return err
	}
	if _, err := w.writer.WriteString(s); err != nil {
		return err
	}
	_, err := w.writer.Write(crlf)
	return err
}

// WriteError writes -error\r\n.
func (w *Writer) WriteError(errStr string) error {
	if err := w.writer.WriteByte(byte(TypeError)); err != nil {
		return err
	}
	if _, err := w.writer.WriteString(errStr); err != nil {
		return err
	}
	_, err := w.writer.Write(crlf)
	return err
}

// WriteInteger writes :number\r\n.
func (w *Writer) WriteInteger(n int64) error {
	if err := w.writer.WriteByte(byte(TypeInteger)); err != nil {
		return err
	}
	if _, err := w.writer.WriteString(strconv.FormatInt(n, 10)); err != nil {
		return err
	}
	_, err := w.writer.Write(crlf)
	return err
}

// WriteBulkString writes $length\r\nbytes\r\n.
func (w *Writer) WriteBulkString(b []byte) error {
	if b == nil {
		return w.WriteNull()
	}
	if err := w.writer.WriteByte(byte(TypeBulkString)); err != nil {
		return err
	}
	if _, err := w.writer.WriteString(strconv.Itoa(len(b))); err != nil {
		return err
	}
	if _, err := w.writer.Write(crlf); err != nil {
		return err
	}
	if _, err := w.writer.Write(b); err != nil {
		return err
	}
	_, err := w.writer.Write(crlf)
	return err
}

// WriteBulkStringString writes a string as a BulkString.
func (w *Writer) WriteBulkStringString(s string) error {
	return w.WriteBulkString([]byte(s))
}

// WriteNull writes Null BulkString ($-1\r\n).
func (w *Writer) WriteNull() error {
	_, err := w.writer.WriteString("$-1\r\n")
	return err
}

// WriteNullArray writes Null Array (*-1\r\n).
func (w *Writer) WriteNullArray() error {
	_, err := w.writer.WriteString("*-1\r\n")
	return err
}

// WriteArray writes *count\r\n followed by elements.
func (w *Writer) WriteArray(values []Value) error {
	if values == nil {
		return w.WriteNullArray()
	}

	if err := w.writer.WriteByte(byte(TypeArray)); err != nil {
		return err
	}
	if _, err := w.writer.WriteString(strconv.Itoa(len(values))); err != nil {
		return err
	}
	if _, err := w.writer.Write(crlf); err != nil {
		return err
	}

	for _, elem := range values {
		if err := w.WriteValue(elem); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes buffered bytes to the underlying io.Writer.
func (w *Writer) Flush() error {
	return w.writer.Flush()
}
