package server

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"vortex-cache/internal/resp"
	"vortex-cache/internal/store"
)

// Handler processes parsed RESP commands against a Store instance.
type Handler struct {
	cache     *store.Store
	startTime time.Time
}

// NewHandler creates a new command handler.
func NewHandler(cache *store.Store, startTime time.Time) *Handler {
	return &Handler{
		cache:     cache,
		startTime: startTime,
	}
}

// Dispatch receives a parsed RESP command array and returns a RESP response.
func (h *Handler) Dispatch(cmd resp.Value) resp.Value {
	if cmd.Type != resp.TypeArray || len(cmd.Array) == 0 {
		return resp.NewError("ERR invalid command format")
	}

	cmdName := strings.ToUpper(h.extractString(cmd.Array[0]))
	args := cmd.Array[1:]

	switch cmdName {
	case "PING":
		return h.handlePing(args)
	case "GET":
		return h.handleGet(args)
	case "SET":
		return h.handleSet(args)
	case "DEL":
		return h.handleDel(args)
	case "EXISTS":
		return h.handleExists(args)
	case "EXPIRE":
		return h.handleExpire(args)
	case "TTL":
		return h.handleTTL(args)
	case "INFO":
		return h.handleInfo(args)
	default:
		return resp.NewError(fmt.Sprintf("ERR unknown command '%s'", cmdName))
	}
}

func (h *Handler) extractString(v resp.Value) string {
	if v.Type == resp.TypeBulkString {
		return string(v.Bulk)
	}
	return v.Str
}

func (h *Handler) handlePing(args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.NewSimpleString("PONG")
	}
	if len(args) == 1 {
		return resp.NewBulkStringFromString(h.extractString(args[0]))
	}
	return resp.NewError("ERR wrong number of arguments for 'ping' command")
}

func (h *Handler) handleGet(args []resp.Value) resp.Value {
	if len(args) != 1 {
		return resp.NewError("ERR wrong number of arguments for 'get' command")
	}

	key := h.extractString(args[0])
	val, found := h.cache.Get(key)
	if !found {
		return resp.NewNullBulkString()
	}
	return resp.NewBulkString(val)
}

func (h *Handler) handleSet(args []resp.Value) resp.Value {
	if len(args) < 2 {
		return resp.NewError("ERR wrong number of arguments for 'set' command")
	}

	key := h.extractString(args[0])
	val := args[1].Bulk
	if val == nil {
		val = []byte(args[1].Str)
	}

	var ttl time.Duration

	// Parse optional EX/PX flags
	for i := 2; i < len(args); i++ {
		flag := strings.ToUpper(h.extractString(args[i]))
		if flag == "EX" || flag == "PX" {
			if i+1 >= len(args) {
				return resp.NewError("ERR syntax error")
			}
			numStr := h.extractString(args[i+1])
			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil || num <= 0 {
				return resp.NewError("ERR value is not an integer or out of range")
			}

			if flag == "EX" {
				ttl = time.Duration(num) * time.Second
			} else {
				ttl = time.Duration(num) * time.Millisecond
			}
			i++ // Skip parsed value
		}
	}

	if err := h.cache.Set(key, val, ttl); err != nil {
		return resp.NewError(fmt.Sprintf("ERR %v", err))
	}

	return resp.NewSimpleString("OK")
}

func (h *Handler) handleDel(args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.NewError("ERR wrong number of arguments for 'del' command")
	}

	var count int64
	for _, arg := range args {
		key := h.extractString(arg)
		if h.cache.Delete(key) {
			count++
		}
	}
	return resp.NewInteger(count)
}

func (h *Handler) handleExists(args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.NewError("ERR wrong number of arguments for 'exists' command")
	}

	var count int64
	for _, arg := range args {
		key := h.extractString(arg)
		if h.cache.Exists(key) {
			count++
		}
	}
	return resp.NewInteger(count)
}

func (h *Handler) handleExpire(args []resp.Value) resp.Value {
	if len(args) != 2 {
		return resp.NewError("ERR wrong number of arguments for 'expire' command")
	}

	key := h.extractString(args[0])
	numStr := h.extractString(args[1])
	seconds, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return resp.NewError("ERR value is not an integer or out of range")
	}

	ttl := time.Duration(seconds) * time.Second
	if h.cache.Expire(key, ttl) {
		return resp.NewInteger(1)
	}
	return resp.NewInteger(0)
}

func (h *Handler) handleTTL(args []resp.Value) resp.Value {
	if len(args) != 1 {
		return resp.NewError("ERR wrong number of arguments for 'ttl' command")
	}

	key := h.extractString(args[0])
	remaining, found := h.cache.TTL(key)
	if !found || remaining == -2 {
		return resp.NewInteger(-2)
	}
	if remaining == -1 {
		return resp.NewInteger(-1)
	}

	sec := int64(remaining.Seconds())
	if sec == 0 && remaining > 0 {
		sec = 1
	}
	return resp.NewInteger(sec)
}

func (h *Handler) handleInfo(args []resp.Value) resp.Value {
	uptimeSec := int64(time.Since(h.startTime).Seconds())
	keysCount := h.cache.Len()

	infoText := fmt.Sprintf("# Server\r\n"+
		"vortex_version:0.1.0\r\n"+
		"go_version:%s\r\n"+
		"process_id:%d\r\n"+
		"uptime_in_seconds:%d\r\n"+
		"\r\n"+
		"# Keyspace\r\n"+
		"%s\r\n"+
		"db0:keys=%d,expires=0\r\n",
		runtime.Version(),
		os.Getpid(),
		uptimeSec,
		h.cache.Stats(),
		keysCount,
	)

	return resp.NewBulkStringFromString(infoText)
}
