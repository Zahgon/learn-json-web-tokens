// Package test is the Go port of the tape suites that lived in
// example/test. The functional suite drives the helpers directly through the
// shared mock request and response, exactly as the original did.
package test

import (
	"testing"
	"time"

	"github.com/dwyl/learn-json-web-tokens/example/lib"
	"github.com/dwyl/learn-json-web-tokens/internal/jsonwebtoken"
	"github.com/dwyl/learn-json-web-tokens/internal/querystring"
)

// secret mirrors the literal the original functional suite hard-coded rather
// than reading it back from the environment.
const secret = "CHANGE_THIS_TO_SOMETHING_RANDOM"

// token carries the credential produced by the "handler" case into the cases
// that follow it, the way the suite-level variable did in the original.
var token any

// TestFunctional runs the fourteen cases of example/test/functional.js in the
// order the original declared them. The order matters: the mock request and
// response are shared singletons that are never reset, so state deposited by
// one case is observed by the next.
func TestFunctional(t *testing.T) {
	index := lib.View("index")
	success := lib.View("restricted")
	fail := lib.View("fail")

	t.Run("home", func(t *testing.T) {
		res := lib.Home(Res).(*MockResponse)
		equalInt(t, 200, 200, "Status 200")
		equal(t, res.Body, index, "Homepage rendered")
	})

	t.Run("fail", func(t *testing.T) {
		res := lib.Fail(Res).(*MockResponse)
		equalInt(t, res.Status, 401, "Status 401")
		equal(t, res.Body, fail, "Rejected (as expected)")
	})

	t.Run("success", func(t *testing.T) {
		res := lib.Success(Req, Res).(*MockResponse)
		equalInt(t, res.Status, 200, "Successfully authenticated")
		equal(t, res.Body, success, "Success.")
	})

	t.Run("handler incorrect username & password", func(t *testing.T) {
		lib.Handler(Req, Res)
		Req.Emit("data", querystring.StringifyPairs([][2]string{
			{"username", "badguy"},
			{"password", "kragle"},
		}))
		Req.Emit("end", nil)
		equalInt(t, Res.Status, 401, "Auth fail")
	})

	t.Run("handler GET", func(t *testing.T) {
		Req.MethodStr = "GET"
		res := lib.Handler(Req, Res).(*MockResponse)
		equalInt(t, res.Status, 401, "GET should fail")
	})

	t.Run("handler", func(t *testing.T) {
		Req.MethodStr = "POST"
		lib.Handler(Req, Res)
		Req.Emit("data", querystring.StringifyPairs([][2]string{
			{"username", "masterbuilder"},
			{"password", "itsnosecret"},
		}))
		Req.Emit("end", nil)
		token = Res.Headers["authorization"]
		equalInt(t, Res.Status, 200, "Authenticated")
	})

	t.Run("validation fail (bad-but-valid token)", func(t *testing.T) {
		bad, err := jsonwebtoken.Sign([]jsonwebtoken.Field{
			{Key: "auth", Value: "invalid"},
			{Key: "agent", Value: Req.Header("user-agent")},
		}, secret, jsonwebtoken.Options{ExpiresIn: "7d"})
		if err != nil {
			t.Fatalf("could not sign the bad token: %v", err)
		}

		Req.Headers["authorization"] = bad
		done := make(chan struct{})
		lib.Validate(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 401, "should NOT validate using BAD token")
			close(done)
		})
		await(t, done)
	})

	t.Run("validation fail (invalid token)", func(t *testing.T) {
		Req.Headers["authorization"] = "malformed token"
		done := make(chan struct{})
		lib.Validate(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 401, "should NOT validate using INVALID token")
			close(done)
		})
		await(t, done)
	})

	t.Run("validate", func(t *testing.T) {
		Req.Headers["authorization"] = token
		done := make(chan struct{})
		lib.Validate(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 200, "should validate using token")
			close(done)
		})
		await(t, done)
	})

	t.Run("logout", func(t *testing.T) {
		done := make(chan struct{})
		lib.Logout(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 200, "Logged out")
			close(done)
		})
		await(t, done)
	})

	t.Run("no access after logout", func(t *testing.T) {
		Req.Headers["authorization"] = token
		done := make(chan struct{})
		lib.Validate(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 401, "No longer has access to private content!")
			close(done)
		})
		await(t, done)
	})

	t.Run("malicious logout", func(t *testing.T) {
		Req.Headers["authorization"] = "malformed token"
		done := make(chan struct{})
		lib.Logout(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 401, "Logged out")
			close(done)
		})
		await(t, done)
	})

	t.Run("notFound", func(t *testing.T) {
		res := lib.NotFound(Res).(*MockResponse)
		equalInt(t, res.Status, 404, "Not found")
	})

	// Declared last in the original so the wait it introduces cannot disturb
	// the cases that share the database.
	t.Run("validation fail (expired token)", func(t *testing.T) {
		expiring := lib.GenerateAndStoreToken(Req, lib.TokenOptions{Expires: "1s"})
		Req.Headers["authorization"] = expiring

		time.Sleep(1100 * time.Millisecond)

		done := make(chan struct{})
		lib.Validate(Req, Res, func(res lib.Response) {
			equalInt(t, res.(*MockResponse).Status, 401, "should NOT validate using EXPIRED token")
			close(done)
		})
		await(t, done)
	})

	// The original closed the module with a timer that called done and exit,
	// which is what gave those two helpers their coverage. exit terminates the
	// process, so the hook it goes through is redirected for the duration.
	exited := false
	restore := lib.ProcessExit
	lib.ProcessExit = func(int) { exited = true }
	defer func() { lib.ProcessExit = restore }()

	lib.Done(Res)
	lib.Exit(Res)

	if !exited {
		t.Error("exit should have asked the process to terminate")
	}
}

// equal reports a mismatch between two strings using the message the original
// assertion carried.
func equal(t *testing.T, actual, expected, message string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: got %q, want %q", message, actual, expected)
	}
}

// equalInt is equal for the status codes.
func equalInt(t *testing.T, actual, expected int, message string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: got %d, want %d", message, actual, expected)
	}
}

// await blocks until a callback has run, so that a case which never calls back
// fails loudly instead of passing by omission.
func await(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the callback was never invoked")
	}
}
