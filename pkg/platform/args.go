package platform

import (
	"fmt"
	"math"
)

// argError describes a malformed argument received from a native platform
// channel. Reporting is the caller's responsibility (see e.g.
// reportPlatformViewArg in platform_view_args.go); these helpers stay pure so
// the same parser works on multiple channels.
type argError struct {
	Op   string
	Key  string
	Want string
	Got  any
}

func (e *argError) Error() string {
	if e.Got == nil {
		return fmt.Sprintf("%s: missing required arg %q (want %s)", e.Op, e.Key, e.Want)
	}
	return fmt.Sprintf("%s: arg %q: want %s, got %T", e.Op, e.Key, e.Want, e.Got)
}

func requireMap(op string, v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return nil, &argError{Op: op, Key: "<args>", Want: "map[string]any", Got: v}
}

func requireString(op string, args map[string]any, key string) (string, error) {
	if s, ok := args[key].(string); ok {
		return s, nil
	}
	return "", &argError{Op: op, Key: key, Want: "string", Got: args[key]}
}

func requireBool(op string, args map[string]any, key string) (bool, error) {
	if b, ok := args[key].(bool); ok {
		return b, nil
	}
	return false, &argError{Op: op, Key: key, Want: "bool", Got: args[key]}
}

// requireInt64 accepts any concrete int/uint type at full int64 range, and
// float32/float64 only when the value is integral and within ±(2^53 - 1) —
// the largest range where every integer is exactly representable as float64.
// JSON numbers arrive as float64, so a payload of 9007199254740993 decodes as
// 9007199254740992 (rounded); rejecting above 2^53-1 prevents silently
// accepting such already-rounded values.
func requireInt64(op string, args map[string]any, key string) (int64, error) {
	v := args[key]
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint:
		if uint64(n) <= uint64(math.MaxInt64) {
			return int64(n), nil
		}
	case uint64:
		if n <= uint64(math.MaxInt64) {
			return int64(n), nil
		}
	case float64:
		if isIntegral(n) && n >= -((1<<53)-1) && n <= (1<<53)-1 {
			return int64(n), nil
		}
	}
	// float32 deliberately omitted: its 24-bit mantissa silently rounds
	// integers above 2^24, and the JSON wire format never produces float32
	// anyway. In-process callers wanting truncation should convert explicitly.
	return 0, &argError{Op: op, Key: key, Want: "int64", Got: v}
}

func requireInt(op string, args map[string]any, key string) (int, error) {
	n, err := requireInt64(op, args, key)
	if err != nil {
		return 0, err
	}
	if n < math.MinInt || n > math.MaxInt {
		return 0, &argError{Op: op, Key: key, Want: "int", Got: args[key]}
	}
	return int(n), nil
}

func requireUint32(op string, args map[string]any, key string) (uint32, error) {
	v := args[key]
	switch n := v.(type) {
	case uint8:
		return uint32(n), nil
	case uint16:
		return uint32(n), nil
	case uint32:
		return n, nil
	case uint:
		if uint64(n) <= math.MaxUint32 {
			return uint32(n), nil
		}
	case uint64:
		if n <= math.MaxUint32 {
			return uint32(n), nil
		}
	case int:
		// On 32-bit GOARCH, math.MaxUint32 is wider than math.MaxInt; cast
		// through uint64 so the comparison is well-defined everywhere.
		if n >= 0 && uint64(n) <= math.MaxUint32 {
			return uint32(n), nil
		}
	case int8:
		if n >= 0 {
			return uint32(n), nil
		}
	case int16:
		if n >= 0 {
			return uint32(n), nil
		}
	case int32:
		if n >= 0 {
			return uint32(n), nil
		}
	case int64:
		if n >= 0 && uint64(n) <= math.MaxUint32 {
			return uint32(n), nil
		}
	case float64:
		if isIntegral(n) && n >= 0 && n <= math.MaxUint32 {
			return uint32(n), nil
		}
	}
	// float32 deliberately omitted; see requireInt64 for rationale.
	return 0, &argError{Op: op, Key: key, Want: "uint32", Got: v}
}

func requireFloat64(op string, args map[string]any, key string) (float64, error) {
	v := args[key]
	var f float64
	switch n := v.(type) {
	case float64:
		f = n
	case float32:
		f = float64(n)
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, &argError{Op: op, Key: key, Want: "float64", Got: v}
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, &argError{Op: op, Key: key, Want: "finite float64", Got: v}
	}
	return f, nil
}

func isIntegral(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && math.Trunc(f) == f
}
