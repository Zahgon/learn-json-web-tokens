// Package level is a minimal port of the `level` module as the original
// implementation uses it: an on-disk key/value store whose default encoding for
// both keys and values is "utf8".
//
// The utf8 encoding is the important part. LevelDB never sees a JavaScript
// value; it sees whatever String() produced. Storing an object therefore stores
// the text "[object Object]", and helpers.js depends on that.
package level

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

// NotFoundError is returned by Get when the key is absent. It mirrors the
// LevelUP error that carries a `notFound` property.
type NotFoundError struct {
	Key string
}

func (e *NotFoundError) Error() string {
	return "Key not found in database [" + e.Key + "]"
}

// NotFound reports that this error represents a missing key.
func (e *NotFoundError) NotFound() bool { return true }

// IsNotFound reports whether err represents a missing key.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

// DB is an open store rooted at a directory.
type DB struct {
	dir string
	mu  sync.RWMutex
}

// Open creates the store directory if needed and returns a handle to it. It is
// the equivalent of level(__dirname + '/db').
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DB{dir: dir}, nil
}

// Dir returns the directory backing the store.
func (d *DB) Dir() string { return d.dir }

// Put writes value under key. Both are coerced with String() first, exactly as
// the utf8 encoding does.
func (d *DB) Put(key, value any) error {
	k := jsvalue.String(key)
	v := jsvalue.String(value)

	d.mu.Lock()
	defer d.mu.Unlock()

	return os.WriteFile(d.path(k), []byte(v), 0o644)
}

// Get reads the value stored under key. A missing key yields a *NotFoundError.
func (d *DB) Get(key any) (string, error) {
	k := jsvalue.String(key)

	d.mu.RLock()
	defer d.mu.RUnlock()

	data, err := os.ReadFile(d.path(k))
	if err != nil {
		if os.IsNotExist(err) {
			return "", &NotFoundError{Key: k}
		}
		return "", err
	}
	return string(data), nil
}

// Del removes a key, ignoring keys that are already absent.
func (d *DB) Del(key any) error {
	k := jsvalue.String(key)

	d.mu.Lock()
	defer d.mu.Unlock()

	err := os.Remove(d.path(k))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// path maps a key to its backing file. Keys are hex encoded so that any byte
// sequence is a legal file name on every platform.
func (d *DB) path(key string) string {
	return filepath.Join(d.dir, hex.EncodeToString([]byte(key))+".val")
}
