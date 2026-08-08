package resp

// Type represents a RESP2 protocol data type prefix byte.
type Type byte

const (
	TypeSimpleString Type = '+'
	TypeError        Type = '-'
	TypeInteger      Type = ':'
	TypeBulkString   Type = '$'
	TypeArray        Type = '*'
)

// Value represents a parsed or encodable RESP2 frame.
type Value struct {
	Type  Type
	Str   string   // Used for SimpleString and Error
	Num   int64    // Used for Integer
	Bulk  []byte   // Used for BulkString
	Array []Value  // Used for Array
	Nil   bool     // Indicates Null BulkString ($-1\r\n) or Null Array (*-1\r\n)
}

// NewSimpleString creates a RESP SimpleString (+OK\r\n).
func NewSimpleString(s string) Value {
	return Value{Type: TypeSimpleString, Str: s}
}

// NewError creates a RESP Error (-ERR message\r\n).
func NewError(err string) Value {
	return Value{Type: TypeError, Str: err}
}

// NewInteger creates a RESP Integer (:100\r\n).
func NewInteger(n int64) Value {
	return Value{Type: TypeInteger, Num: n}
}

// NewBulkString creates a RESP BulkString ($5\r\nhello\r\n).
func NewBulkString(b []byte) Value {
	if b == nil {
		return Value{Type: TypeBulkString, Nil: true}
	}
	valCopy := make([]byte, len(b))
	copy(valCopy, b)
	return Value{Type: TypeBulkString, Bulk: valCopy}
}

// NewBulkStringBytes creates a RESP BulkString directly without copying slice.
func NewBulkStringBytes(b []byte) Value {
	if b == nil {
		return Value{Type: TypeBulkString, Nil: true}
	}
	return Value{Type: TypeBulkString, Bulk: b}
}

// NewBulkStringFromString creates a RESP BulkString from string.
func NewBulkStringFromString(s string) Value {
	return Value{Type: TypeBulkString, Bulk: []byte(s)}
}

// NewNullBulkString creates a Null BulkString ($-1\r\n).
func NewNullBulkString() Value {
	return Value{Type: TypeBulkString, Nil: true}
}

// NewArray creates a RESP Array (*2\r\n...).
func NewArray(values []Value) Value {
	if values == nil {
		return Value{Type: TypeArray, Nil: true}
	}
	return Value{Type: TypeArray, Array: values}
}
