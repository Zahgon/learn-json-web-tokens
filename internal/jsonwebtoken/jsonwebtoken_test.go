package jsonwebtoken

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

const secret = "CHANGE_THIS_TO_SOMETHING_RANDOM"

// The header is a fixed object, so its segment is a constant for every token
// the helpers issue.
const headerSegment = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

func freeze(t *testing.T, at time.Time) {
	t.Helper()

	previous := Now
	Now = func() time.Time { return at }
	t.Cleanup(func() { Now = previous })
}

func segments(t *testing.T, token string) []string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}
	return parts
}

func decodeSegment(t *testing.T, segment string) string {
	t.Helper()

	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("segment is not base64url: %v", err)
	}
	return string(raw)
}

func TestSignEmitsAHeaderPayloadAndSignature(t *testing.T) {
	token, err := Sign([]Field{{Key: "auth", Value: int64(1)}}, secret, Options{})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := segments(t, token)
	if parts[0] != headerSegment {
		t.Errorf("header segment = %q, want %q", parts[0], headerSegment)
	}
	if decodeSegment(t, parts[0]) != `{"alg":"HS256","typ":"JWT"}` {
		t.Errorf("header = %q", decodeSegment(t, parts[0]))
	}
	if parts[2] == "" {
		t.Error("signature segment is empty")
	}
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q is not base64url without padding", token)
	}
}

// The claims are emitted in the order they are supplied, followed by iat and
// then exp. A map would sort them and change the bytes on the wire.
func TestSignPreservesClaimOrder(t *testing.T) {
	issued := time.Unix(1787724658, 0)
	freeze(t, issued)

	token, err := Sign([]Field{
		{Key: "auth", Value: int64(1787724658777)},
		{Key: "agent", Value: "Mozilla/5.0"},
	}, secret, Options{ExpiresIn: "7d"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	want := `{"auth":1787724658777,"agent":"Mozilla/5.0","iat":1787724658,"exp":1788329458}`
	if payload := decodeSegment(t, segments(t, token)[1]); payload != want {
		t.Errorf("payload = %q, want %q", payload, want)
	}
}

func TestSignExpiresIn(t *testing.T) {
	cases := []struct {
		expiresIn any
		want      float64
	}{
		{"7d", 604800},
		{"1s", 1},
		{"1 second", 1},
		{"2m", 120},
		{"1h", 3600},
		{"1w", 604800},
		{100, 100},
	}

	for _, c := range cases {
		token, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{ExpiresIn: c.expiresIn})
		if err != nil {
			t.Fatalf("Sign(%v): %v", c.expiresIn, err)
		}

		claims, err := Verify(token, secret)
		if err != nil {
			t.Fatalf("Verify(%v): %v", c.expiresIn, err)
		}

		iat, ok := claims["iat"].(float64)
		if !ok {
			t.Fatalf("iat is %T, want a number", claims["iat"])
		}
		exp, ok := claims["exp"].(float64)
		if !ok {
			t.Fatalf("exp is %T, want a number", claims["exp"])
		}
		if exp-iat != c.want {
			t.Errorf("exp-iat for %v = %v, want %v", c.expiresIn, exp-iat, c.want)
		}
	}
}

func TestSignWithoutExpiresInOmitsExp(t *testing.T) {
	token, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	claims, err := Verify(token, secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, present := claims["exp"]; present {
		t.Error("exp was emitted without an expiry")
	}
	if _, present := claims["iat"]; !present {
		t.Error("iat was not emitted")
	}
}

func TestSignNoTimestampOmitsIat(t *testing.T) {
	token, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{NoTimestamp: true})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if payload := decodeSegment(t, segments(t, token)[1]); payload != `{"auth":1}` {
		t.Errorf("payload = %q, want %q", payload, `{"auth":1}`)
	}
}

// An undefined claim is dropped rather than serialised as null, which is what
// happens when a request arrives with no user-agent header.
func TestSignDropsUndefinedClaims(t *testing.T) {
	token, err := Sign([]Field{
		{Key: "auth", Value: 1},
		{Key: "agent", Value: jsvalue.Undefined},
	}, secret, Options{NoTimestamp: true})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if payload := decodeSegment(t, segments(t, token)[1]); payload != `{"auth":1}` {
		t.Errorf("payload = %q, want %q", payload, `{"auth":1}`)
	}
}

func TestSignRejectsAnUnusableExpiresIn(t *testing.T) {
	if _, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{ExpiresIn: "tomorrow"}); err == nil {
		t.Error("Sign accepted an unparseable expiry")
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	token, err := Sign([]Field{
		{Key: "auth", Value: int64(1787724658777)},
		{Key: "agent", Value: "Mozilla/5.0"},
	}, secret, Options{ExpiresIn: "7d"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	claims, err := Verify(token, secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if auth, ok := claims["auth"].(float64); !ok || auth != 1787724658777 {
		t.Errorf("auth = %v (%T), want 1787724658777", claims["auth"], claims["auth"])
	}
	if claims["agent"] != "Mozilla/5.0" {
		t.Errorf("agent = %v, want Mozilla/5.0", claims["agent"])
	}
}

func TestVerifyStringClaimsSurviveTheRoundTrip(t *testing.T) {
	token, err := Sign([]Field{{Key: "auth", Value: "invalid"}}, secret, Options{ExpiresIn: "7d"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	claims, err := Verify(token, secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims["auth"] != "invalid" {
		t.Errorf("auth = %v, want the string invalid", claims["auth"])
	}
}

func TestVerifyRejections(t *testing.T) {
	valid, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{ExpiresIn: "7d"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	parts := segments(t, valid)

	cases := []struct {
		name    string
		token   string
		secret  string
		message string
	}{
		{"empty", "", secret, "jwt must be provided"},
		{"one segment", "malformed", secret, "jwt malformed"},
		{"two segments", parts[0] + "." + parts[1], secret, "jwt malformed"},
		{"four segments", valid + ".extra", secret, "jwt malformed"},
		{"wrong secret", valid, "another secret", "invalid signature"},
		{"tampered signature", parts[0] + "." + parts[1] + ".AAAA", secret, "invalid signature"},
		{"unreadable header", "!!!." + parts[1] + "." + parts[2], secret, "invalid token"},
		{"unreadable payload", parts[0] + ".!!!." + parts[2], secret, "invalid token"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			claims, err := Verify(c.token, c.secret)
			if err == nil {
				t.Fatalf("Verify accepted %q", c.token)
			}
			if claims != nil {
				t.Errorf("Verify returned claims alongside an error")
			}
			if err.Error() != c.message {
				t.Errorf("Error() = %q, want %q", err.Error(), c.message)
			}
			if IsExpired(err) {
				t.Error("IsExpired = true, want false")
			}
		})
	}
}

func TestVerifyRejectsAnExpiredToken(t *testing.T) {
	issued := time.Unix(1787724658, 0)
	freeze(t, issued)

	token, err := Sign([]Field{{Key: "auth", Value: 1}}, secret, Options{ExpiresIn: "1s"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if _, err := Verify(token, secret); err != nil {
		t.Fatalf("Verify before the expiry: %v", err)
	}

	Now = func() time.Time { return issued.Add(2 * time.Second) }

	claims, err := Verify(token, secret)
	if err == nil {
		t.Fatal("Verify accepted an expired token")
	}
	if claims != nil {
		t.Error("Verify returned claims for an expired token")
	}
	if err.Error() != "jwt expired" {
		t.Errorf("Error() = %q, want %q", err.Error(), "jwt expired")
	}
	if !IsExpired(err) {
		t.Error("IsExpired = false, want true")
	}
}

func TestErrorFields(t *testing.T) {
	expiredAt := time.Unix(1787724659, 0)
	err := &Error{Name: "TokenExpiredError", Message: "jwt expired", ExpiredAt: expiredAt}

	if !err.Expired() {
		t.Error("Expired() = false, want true")
	}
	if err.Error() != "jwt expired" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !err.ExpiredAt.Equal(expiredAt) {
		t.Errorf("ExpiredAt = %v, want %v", err.ExpiredAt, expiredAt)
	}

	plain := &Error{Name: "JsonWebTokenError", Message: "invalid token"}
	if plain.Expired() {
		t.Error("Expired() = true for a non-expiry error")
	}
	if IsExpired(nil) {
		t.Error("IsExpired(nil) = true, want false")
	}
}

func TestDecodeSkipsVerification(t *testing.T) {
	token, err := Sign([]Field{{Key: "auth", Value: "magic"}}, secret, Options{NoTimestamp: true})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	tampered := segments(t, token)[0] + "." + segments(t, token)[1] + ".AAAA"

	claims, err := Decode(tampered)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if claims["auth"] != "magic" {
		t.Errorf("auth = %v, want magic", claims["auth"])
	}

	if _, err := Decode("malformed"); err == nil {
		t.Error("Decode accepted a malformed token")
	}
}
