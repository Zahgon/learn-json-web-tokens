package level

import (
	"os"
	"path/filepath"
	"testing"
)

func open(t *testing.T) *DB {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

func TestOpenCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "db")

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if db.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", db.Dir(), dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("Open did not create a directory")
	}
}

func TestPutAndGet(t *testing.T) {
	db := open(t)

	if err := db.Put("greeting", "hello"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	value, err := db.Get("greeting")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "hello" {
		t.Errorf("Get = %q, want %q", value, "hello")
	}
}

func TestPutOverwrites(t *testing.T) {
	db := open(t)

	if err := db.Put("k", "first"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Put("k", "second"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	value, err := db.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "second" {
		t.Errorf("Get = %q, want %q", value, "second")
	}
}

// Keys go through the same utf8 encoding as values, so a numeric key and its
// decimal string are the same record. The helpers store tokens under a
// millisecond timestamp and read them back from a JSON claim.
func TestNumericKeysAreTheirDecimalString(t *testing.T) {
	db := open(t)

	if err := db.Put(int64(1234567890123), "token"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	for _, key := range []any{int64(1234567890123), "1234567890123", float64(1234567890123)} {
		value, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get(%v): %v", key, err)
		}
		if value != "token" {
			t.Errorf("Get(%v) = %q, want %q", key, value, "token")
		}
	}
}

// Storing anything other than a string or a number renders it through the same
// coercion the utf8 encoding applies, which is what revokes a token on logout.
func TestPutOfAnObjectStoresObjectObject(t *testing.T) {
	db := open(t)

	record := map[string]any{"valid": false, "created": 1234567890123}
	if err := db.Put("session", record); err != nil {
		t.Fatalf("Put: %v", err)
	}

	value, err := db.Get("session")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "[object Object]" {
		t.Errorf("Get = %q, want %q", value, "[object Object]")
	}
}

func TestPutOfNumbersAndBooleans(t *testing.T) {
	db := open(t)

	cases := []struct {
		key   string
		value any
		want  string
	}{
		{"int", 42, "42"},
		{"float", 1.5, "1.5"},
		{"integral-float", float64(7), "7"},
		{"bool", true, "true"},
		{"nil", nil, "null"},
		{"slice", []string{"a", "b"}, "a,b"},
	}

	for _, c := range cases {
		if err := db.Put(c.key, c.value); err != nil {
			t.Fatalf("Put(%s): %v", c.key, err)
		}
		value, err := db.Get(c.key)
		if err != nil {
			t.Fatalf("Get(%s): %v", c.key, err)
		}
		if value != c.want {
			t.Errorf("Get(%s) = %q, want %q", c.key, value, c.want)
		}
	}
}

func TestGetOfAMissingKey(t *testing.T) {
	db := open(t)

	value, err := db.Get("absent")
	if err == nil {
		t.Fatal("Get of a missing key returned no error")
	}
	if value != "" {
		t.Errorf("Get returned %q alongside an error", value)
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}

	want := "Key not found in database [absent]"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestNotFoundErrorReportsItself(t *testing.T) {
	err := &NotFoundError{Key: "k"}

	if !err.NotFound() {
		t.Error("NotFound() = false, want true")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true, want false")
	}
	if IsNotFound(os.ErrClosed) {
		t.Error("IsNotFound of an unrelated error = true, want false")
	}
}

func TestDel(t *testing.T) {
	db := open(t)

	if err := db.Put("k", "v"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Del("k"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if _, err := db.Get("k"); !IsNotFound(err) {
		t.Errorf("Get after Del: %v, want a not found error", err)
	}
}

func TestDelOfAMissingKeyIsNotAnError(t *testing.T) {
	db := open(t)

	if err := db.Del("absent"); err != nil {
		t.Errorf("Del of a missing key: %v", err)
	}
}

func TestKeysMayContainPathSeparators(t *testing.T) {
	db := open(t)

	keys := []string{"a/b", `c\d`, "..", "with space", "unicode-\u00e9"}
	for _, key := range keys {
		if err := db.Put(key, key); err != nil {
			t.Fatalf("Put(%q): %v", key, err)
		}
	}

	for _, key := range keys {
		value, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		if value != key {
			t.Errorf("Get(%q) = %q, want %q", key, value, key)
		}
	}
}
