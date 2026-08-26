// Package jsonwebtoken is a port of the subset of the `jsonwebtoken` module
// that the original implementation uses: HS256 signing with an expiry, and
// verification that rejects malformed, badly signed and expired tokens.
//
// Error names and messages are preserved because they are part of the observed
// behaviour, even though helpers.js collapses every failure to `false`.
package jsonwebtoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

// Now is the clock used for `iat`, `exp` and expiry checks. Tests may replace
// it; production code leaves it alone.
var Now = time.Now

// Field is one payload entry. A slice of them preserves key order, which
// JavaScript objects give for free and Go maps do not.
type Field struct {
	Key   string
	Value any
}

// Claims is a decoded payload.
type Claims map[string]any

// Options mirrors the sign options this project passes.
type Options struct {
	// ExpiresIn is either a string in `ms` format ("7d", "1s") or a number of
	// seconds. A nil value means no `exp` claim is added.
	ExpiresIn any
	// NoTimestamp suppresses the `iat` claim.
	NoTimestamp bool
}

// Error is the equivalent of JsonWebTokenError and TokenExpiredError.
type Error struct {
	Name      string
	Message   string
	ExpiredAt time.Time
}

func (e *Error) Error() string { return e.Message }

// Expired reports whether the error is a TokenExpiredError.
func (e *Error) Expired() bool { return e.Name == "TokenExpiredError" }

func newError(message string) *Error {
	return &Error{Name: "JsonWebTokenError", Message: message}
}

func newExpiredError(expiredAt time.Time) *Error {
	return &Error{Name: "TokenExpiredError", Message: "jwt expired", ExpiredAt: expiredAt}
}

// IsExpired reports whether err is a TokenExpiredError.
func IsExpired(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Expired()
}

var b64 = base64.RawURLEncoding

// Sign produces an HS256 token. The payload is emitted in the order given,
// followed by `iat` and then `exp`, matching the reference implementation.
func Sign(payload []Field, secret string, opts Options) (string, error) {
	timestamp := Now().Unix()

	fields := make([]Field, 0, len(payload)+2)
	fields = append(fields, payload...)

	if !opts.NoTimestamp {
		fields = append(fields, Field{Key: "iat", Value: timestamp})
	}
	if opts.ExpiresIn != nil {
		exp, err := timespan(opts.ExpiresIn, timestamp)
		if err != nil {
			return "", err
		}
		fields = append(fields, Field{Key: "exp", Value: exp})
	}

	headerJSON, err := encodeObject([]Field{
		{Key: "alg", Value: "HS256"},
		{Key: "typ", Value: "JWT"},
	})
	if err != nil {
		return "", err
	}
	payloadJSON, err := encodeObject(fields)
	if err != nil {
		return "", err
	}

	signingInput := b64.EncodeToString(headerJSON) + "." + b64.EncodeToString(payloadJSON)
	return signingInput + "." + b64.EncodeToString(mac(signingInput, secret)), nil
}

// Verify checks the signature and the expiry of a token and returns its claims.
func Verify(token, secret string) (Claims, error) {
	if token == "" {
		return nil, newError("jwt must be provided")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, newError("jwt malformed")
	}

	headerJSON, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, newError("invalid token")
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, newError("invalid token")
	}

	payloadJSON, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, newError("invalid token")
	}
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, newError("invalid token")
	}

	signature, err := b64.DecodeString(parts[2])
	if err != nil {
		return nil, newError("invalid signature")
	}
	if !hmac.Equal(signature, mac(parts[0]+"."+parts[1], secret)) {
		return nil, newError("invalid signature")
	}

	clock := Now().Unix()

	if nbf, ok := numberClaim(claims, "nbf"); ok && clock < int64(nbf) {
		return nil, newError("jwt not active")
	}
	if exp, ok := numberClaim(claims, "exp"); ok && clock >= int64(exp) {
		return nil, newExpiredError(time.Unix(int64(exp), 0))
	}

	return claims, nil
}

// Decode reads the claims without verifying anything.
func Decode(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, newError("jwt malformed")
	}
	payloadJSON, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, newError("invalid token")
	}
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, newError("invalid token")
	}
	return claims, nil
}

func mac(signingInput, secret string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signingInput))
	return h.Sum(nil)
}

func numberClaim(claims Claims, name string) (float64, bool) {
	value, present := claims[name]
	if !present {
		return 0, false
	}
	number, ok := value.(float64)
	if !ok {
		return 0, false
	}
	return number, true
}

// encodeObject marshals ordered fields into a JSON object. Fields whose value
// is undefined are omitted, exactly as JSON.stringify drops them.
func encodeObject(fields []Field) ([]byte, error) {
	var out strings.Builder
	out.WriteByte('{')

	first := true
	for _, field := range fields {
		if field.Value == jsvalue.Undefined {
			continue
		}
		encoded, err := json.Marshal(field.Value)
		if err != nil {
			return nil, err
		}
		if !first {
			out.WriteByte(',')
		}
		first = false

		key, err := json.Marshal(field.Key)
		if err != nil {
			return nil, err
		}
		out.Write(key)
		out.WriteByte(':')
		out.Write(encoded)
	}

	out.WriteByte('}')
	return []byte(out.String()), nil
}

// timespan mirrors the reference implementation's expiry handling: a number is
// a count of seconds, a string is parsed with `ms` semantics.
func timespan(expiresIn any, iat int64) (int64, error) {
	switch value := expiresIn.(type) {
	case string:
		milliseconds, err := parseMS(value)
		if err != nil {
			return 0, err
		}
		return iat + int64(math.Floor(milliseconds/1000)), nil
	case int:
		return iat + int64(value), nil
	case int64:
		return iat + value, nil
	case float64:
		return iat + int64(value), nil
	default:
		return 0, fmt.Errorf(
			`"expiresIn" should be a number of seconds or string representing a timespan`,
		)
	}
}

var msPattern = regexp.MustCompile(
	`^(-?(?:\d+)?\.?\d+) *(milliseconds?|msecs?|ms|seconds?|secs?|s|minutes?|mins?|m|hours?|hrs?|h|days?|d|weeks?|w|years?|yrs?|y)?$`,
)

// parseMS ports the `ms` module's string form, returning milliseconds.
func parseMS(value string) (float64, error) {
	match := msPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if match == nil {
		return 0, fmt.Errorf("invalid timespan %q", value)
	}

	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timespan %q", value)
	}

	const (
		second = 1000.0
		minute = second * 60
		hour   = minute * 60
		day    = hour * 24
		week   = day * 7
		year   = day * 365.25
	)

	switch match[2] {
	case "years", "year", "yrs", "yr", "y":
		return amount * year, nil
	case "weeks", "week", "w":
		return amount * week, nil
	case "days", "day", "d":
		return amount * day, nil
	case "hours", "hour", "hrs", "hr", "h":
		return amount * hour, nil
	case "minutes", "minute", "mins", "min", "m":
		return amount * minute, nil
	case "seconds", "second", "secs", "sec", "s":
		return amount * second, nil
	default:
		return amount, nil
	}
}
