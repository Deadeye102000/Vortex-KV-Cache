package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

var (
	ErrInvalidSyntax = errors.New("ERR invalid RESP syntax")
	ErrUnterminated  = errors.New("ERR unterminated RESP line")
)

// Reader parses incoming RESP2 wire protocol streams.
type Reader struct {
	reader *bufio.Reader
}

// NewReader wraps an io.Reader (such as a net.Conn) in a RESP streaming reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader: bufio.NewReader(r),
	}
}

// ReadValue parses the next complete RESP frame from the underlying stream.
func (r *Reader) ReadValue() (Value, error) {
	prefix, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch Type(prefix) {
	case TypeSimpleString:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return NewSimpleString(string(line)), nil

	case TypeError:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return NewError(string(line)), nil

	case TypeInteger:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("%w: invalid integer format", ErrInvalidSyntax)
		}
		return NewInteger(n), nil

	case TypeBulkString:
		return r.readBulk()

	case TypeArray:
		return r.readArray()

	default:
		return Value{}, fmt.Errorf("%w: unknown prefix '%c'", ErrInvalidSyntax, prefix)
	}
}

// readLine reads bytes until \r\n and returns the line content excluding \r\n.
func (r *Reader) readLine() ([]byte, error) {
	line, err := r.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, ErrUnterminated
	}
	return line[:len(line)-2], nil
}

// readBulk parses a BulkString ($len\r\ndata\r\n).
func (r *Reader) readBulk() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, fmt.Errorf("%w: invalid bulk length", ErrInvalidSyntax)
	}

	if length < 0 {
		return NewNullBulkString(), nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r.reader, buf); err != nil {
		return Value{}, err
	}

	// Read trailing \r\n
	crlf := make([]byte, 2)
	if _, err := io.ReadFull(r.reader, crlf); err != nil {
		return Value{}, err
	}
	if crlf[0] != '\r' || crlf[1] != '\n' {
		return Value{}, fmt.Errorf("%w: expected CRLF after bulk data", ErrInvalidSyntax)
	}

	return NewBulkStringBytes(buf), nil
}

// readArray parses an Array (*count\r\nelem1\r\nelem2...).
func (r *Reader) readArray() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	count, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, fmt.Errorf("%w: invalid array count", ErrInvalidSyntax)
	}

	if count < 0 {
		return NewArray(nil), nil
	}

	elements := make([]Value, count)
	for i := 0; i < count; i++ {
		val, err := r.ReadValue()
		if err != nil {
			return Value{}, err
		}
		elements[i] = val
	}

	return NewArray(elements), nil
}
