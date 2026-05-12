package platform

import (
	"errors"
	"math"
	"testing"
)

func TestRequireMap(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		m, err := requireMap("op", map[string]any{"k": 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["k"] != 1 {
			t.Fatalf("got %v", m)
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := requireMap("op", "not a map")
		assertArgError(t, err, "<args>", "map[string]any")
	})
	t.Run("nil", func(t *testing.T) {
		_, err := requireMap("op", nil)
		assertArgError(t, err, "<args>", "map[string]any")
	})
}

func TestRequireString(t *testing.T) {
	args := map[string]any{
		"good":    "hello",
		"empty":   "",
		"int":     42,
		"bool":    true,
		"bytes":   []byte("not a string"),
		"strType": stringer{},
	}
	t.Run("happy", func(t *testing.T) {
		got, err := requireString("op", args, "good")
		if err != nil || got != "hello" {
			t.Fatalf("got (%q, %v)", got, err)
		}
	})
	t.Run("empty string is valid", func(t *testing.T) {
		got, err := requireString("op", args, "empty")
		if err != nil || got != "" {
			t.Fatalf("got (%q, %v)", got, err)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		_, err := requireString("op", args, "absent")
		assertArgError(t, err, "absent", "string")
	})
	t.Run("wrong type rejected", func(t *testing.T) {
		for _, k := range []string{"int", "bool", "bytes", "strType"} {
			_, err := requireString("op", args, k)
			assertArgError(t, err, k, "string")
		}
	})
}

func TestRequireBool(t *testing.T) {
	args := map[string]any{
		"t":     true,
		"f":     false,
		"sTrue": "true",
		"i":     1,
	}
	t.Run("true", func(t *testing.T) {
		got, err := requireBool("op", args, "t")
		if err != nil || got != true {
			t.Fatalf("got (%v, %v)", got, err)
		}
	})
	t.Run("false", func(t *testing.T) {
		got, err := requireBool("op", args, "f")
		if err != nil || got != false {
			t.Fatalf("got (%v, %v)", got, err)
		}
	})
	t.Run("string 'true' rejected", func(t *testing.T) {
		_, err := requireBool("op", args, "sTrue")
		assertArgError(t, err, "sTrue", "bool")
	})
	t.Run("int 1 rejected", func(t *testing.T) {
		_, err := requireBool("op", args, "i")
		assertArgError(t, err, "i", "bool")
	})
	t.Run("missing", func(t *testing.T) {
		_, err := requireBool("op", args, "absent")
		assertArgError(t, err, "absent", "bool")
	})
}

func TestRequireInt64(t *testing.T) {
	t.Run("native ints round-trip exactly", func(t *testing.T) {
		cases := map[string]any{
			"int":    int(123),
			"int8":   int8(-12),
			"int16":  int16(-1234),
			"int32":  int32(-123456),
			"int64":  int64(math.MaxInt64),
			"uint8":  uint8(200),
			"uint16": uint16(50000),
			"uint32": uint32(math.MaxUint32),
			"uint":   uint(42),
			"uint64": uint64(math.MaxInt64),
		}
		wants := map[string]int64{
			"int":    123,
			"int8":   -12,
			"int16":  -1234,
			"int32":  -123456,
			"int64":  math.MaxInt64,
			"uint8":  200,
			"uint16": 50000,
			"uint32": math.MaxUint32,
			"uint":   42,
			"uint64": math.MaxInt64,
		}
		for k, want := range wants {
			got, err := requireInt64("op", cases, k)
			if err != nil {
				t.Errorf("%s: %v", k, err)
				continue
			}
			if got != want {
				t.Errorf("%s: got %d want %d", k, got, want)
			}
		}
	})
	t.Run("float64 integral within 2^53", func(t *testing.T) {
		got, err := requireInt64("op", map[string]any{"v": float64(123456789)}, "v")
		if err != nil || got != 123456789 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("float64 2^53-1 accepted", func(t *testing.T) {
		got, err := requireInt64("op", map[string]any{"v": float64((1 << 53) - 1)}, "v")
		if err != nil || got != (1<<53)-1 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("float64 above 2^53-1 rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": float64(1 << 53)}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("non-integral float rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": 1.5}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("NaN rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": math.NaN()}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("+Inf rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": math.Inf(1)}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("uint64 above MaxInt64 rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": uint64(math.MaxInt64) + 1}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("float32 rejected (mantissa too narrow)", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": float32(1)}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("wrong type rejected", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{"v": "123"}, "v")
		assertArgError(t, err, "v", "int64")
	})
	t.Run("missing", func(t *testing.T) {
		_, err := requireInt64("op", map[string]any{}, "v")
		assertArgError(t, err, "v", "int64")
	})
}

func TestRequireInt(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		got, err := requireInt("op", map[string]any{"v": 42}, "v")
		if err != nil || got != 42 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("float64 in range", func(t *testing.T) {
		got, err := requireInt("op", map[string]any{"v": float64(7)}, "v")
		if err != nil || got != 7 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("non-integral float rejected", func(t *testing.T) {
		_, err := requireInt("op", map[string]any{"v": 1.5}, "v")
		assertArgError(t, err, "v", "int64")
	})
}

func TestRequireUint32(t *testing.T) {
	t.Run("native uints", func(t *testing.T) {
		got, err := requireUint32("op", map[string]any{"v": uint32(math.MaxUint32)}, "v")
		if err != nil || got != math.MaxUint32 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("float64 MaxUint32 accepted", func(t *testing.T) {
		got, err := requireUint32("op", map[string]any{"v": float64(math.MaxUint32)}, "v")
		if err != nil || got != math.MaxUint32 {
			t.Fatalf("got (%d, %v)", got, err)
		}
	})
	t.Run("float64 MaxUint32+1 rejected", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": float64(math.MaxUint32) + 1}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("negative int rejected", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": -1}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("negative float rejected", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": -0.5}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("non-integral float rejected", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": 1.5}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("int64 above MaxUint32 rejected", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": int64(math.MaxUint32) + 1}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("float32 rejected (mantissa too narrow)", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{"v": float32(1)}, "v")
		assertArgError(t, err, "v", "uint32")
	})
	t.Run("missing", func(t *testing.T) {
		_, err := requireUint32("op", map[string]any{}, "v")
		assertArgError(t, err, "v", "uint32")
	})
}

func TestRequireFloat64(t *testing.T) {
	t.Run("float64", func(t *testing.T) {
		got, err := requireFloat64("op", map[string]any{"v": 3.14}, "v")
		if err != nil || got != 3.14 {
			t.Fatalf("got (%v, %v)", got, err)
		}
	})
	t.Run("int converted", func(t *testing.T) {
		got, err := requireFloat64("op", map[string]any{"v": 42}, "v")
		if err != nil || got != 42.0 {
			t.Fatalf("got (%v, %v)", got, err)
		}
	})
	t.Run("NaN rejected", func(t *testing.T) {
		_, err := requireFloat64("op", map[string]any{"v": math.NaN()}, "v")
		assertArgError(t, err, "v", "finite float64")
	})
	t.Run("+Inf rejected", func(t *testing.T) {
		_, err := requireFloat64("op", map[string]any{"v": math.Inf(1)}, "v")
		assertArgError(t, err, "v", "finite float64")
	})
	t.Run("-Inf rejected", func(t *testing.T) {
		_, err := requireFloat64("op", map[string]any{"v": math.Inf(-1)}, "v")
		assertArgError(t, err, "v", "finite float64")
	})
	t.Run("string rejected", func(t *testing.T) {
		_, err := requireFloat64("op", map[string]any{"v": "3.14"}, "v")
		assertArgError(t, err, "v", "float64")
	})
	t.Run("missing", func(t *testing.T) {
		_, err := requireFloat64("op", map[string]any{}, "v")
		assertArgError(t, err, "v", "float64")
	})
}

func TestIsIntegral(t *testing.T) {
	cases := []struct {
		f    float64
		want bool
	}{
		{0, true},
		{1, true},
		{-1, true},
		{1.5, false},
		{1e10, true},
		{math.NaN(), false},
		{math.Inf(1), false},
		{math.Inf(-1), false},
	}
	for _, c := range cases {
		if got := isIntegral(c.f); got != c.want {
			t.Errorf("isIntegral(%v) = %v, want %v", c.f, got, c.want)
		}
	}
}

func TestArgErrorMessage(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		e := &argError{Op: "myOp", Key: "viewId", Want: "int64", Got: nil}
		want := `myOp: missing required arg "viewId" (want int64)`
		if e.Error() != want {
			t.Errorf("got %q want %q", e.Error(), want)
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		e := &argError{Op: "myOp", Key: "viewId", Want: "int64", Got: "x"}
		want := `myOp: arg "viewId": want int64, got string`
		if e.Error() != want {
			t.Errorf("got %q want %q", e.Error(), want)
		}
	})
}

// helpers

func assertArgError(t *testing.T, err error, wantKey, wantWant string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var ae *argError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *argError, got %T: %v", err, err)
	}
	if ae.Key != wantKey {
		t.Errorf("Key = %q, want %q", ae.Key, wantKey)
	}
	if ae.Want != wantWant {
		t.Errorf("Want = %q, want %q", ae.Want, wantWant)
	}
}

type stringer struct{}

func (stringer) String() string { return "stringer-value" }
