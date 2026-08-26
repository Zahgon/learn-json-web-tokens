// Package lib is the Go port of example/lib/helpers.js.
//
// Every exported name here corresponds to one of the keys the original
// module.exports object published, and the request handling keeps the
// continuation-passing shape of the original rather than adopting the
// net/http handler signature, because the callers and the test suite both
// observe when (and whether) the callback runs.
package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dwyl/learn-json-web-tokens/example/views"
	"github.com/dwyl/learn-json-web-tokens/internal/jsonwebtoken"
	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
	"github.com/dwyl/learn-json-web-tokens/internal/level"
	qs "github.com/dwyl/learn-json-web-tokens/internal/querystring"
)

// Response mirrors the subset of the node http.ServerResponse surface that
// helpers.js touches. Both methods return the response so the original's
// "return res.end(...)" tail calls can be reproduced verbatim.
type Response interface {
	WriteHead(status int, headers map[string]string) Response
	End(body string) Response
}

// Request mirrors the subset of the node http.IncomingMessage surface that
// helpers.js touches.
//
// Header returns any rather than string so that an absent header can be
// reported as jsvalue.Undefined. The distinction matters: a missing
// user-agent has to drop the agent claim from the signed payload the way
// JSON.stringify drops undefined values, and a missing authorization header
// has to reach Verify as undefined rather than as an empty string.
type Request interface {
	Method() string
	Header(name string) any
	OnData(fn func(chunk string)) Request
	OnEnd(fn func()) Request
}

// Callback is the continuation the router hands to Validate and Logout.
type Callback func(res Response)

// TokenOptions is the opts argument of generateToken/generateAndStoreToken.
// A nil Expires selects the seven day default.
type TokenOptions struct {
	Expires any
}

// ProcessExit stands in for process.exit. It is a variable so the test suite
// can observe the call instead of tearing down the test binary, which is the
// single side effect that cannot be reproduced literally and still leave a
// running process behind to report results.
var ProcessExit = os.Exit

var (
	db     *level.DB
	secret string

	index      string
	restricted string
	fail       string
)

func init() {
	var err error
	if db, err = level.Open(filepath.Join(moduleDir(), "db")); err != nil {
		panic(err)
	}

	if secret = os.Getenv("JWT_SECRET"); secret == "" {
		secret = "CHANGE_THIS_TO_SOMETHING_RANDOM"
	}

	index = View("index")
	restricted = View("restricted")
	fail = View("fail")
}

// moduleDir reproduces __dirname: the directory holding this source file.
// The db has to land in example/lib/db exactly as it does in the original,
// including when the tests run from a different working directory.
func moduleDir() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		return filepath.Dir(file)
	}

	executable, err := os.Executable()
	if err != nil {
		return "."
	}

	return filepath.Dir(executable)
}

// View is the exported loadView. The original read the file from disk on
// every call; reading from the embedded copy keeps the same bytes while
// letting a compiled binary run outside the source tree.
func View(view string) string {
	contents, err := views.FS.ReadFile(view + ".html")
	if err != nil {
		panic(err)
	}

	return string(contents)
}

// Fail is authFail. The second parameter exists because logout calls
// authFail(res, done); the original declares it and never invokes it, so
// neither does this.
func Fail(res Response, _ ...Callback) Response {
	res.WriteHead(401, map[string]string{"content-type": "text/html"})
	return res.End(fail)
}

// generateGUID returns milliseconds since the epoch, as a number. The value
// is later used as a database key, where it is coerced to its decimal string
// form, and as the auth claim, where it stays a JSON number.
func generateGUID() int64 {
	return time.Now().UnixMilli()
}

func generateToken(req Request, guid int64, opts TokenOptions) (string, error) {
	const expiresDefault = "7d"

	expires := opts.Expires
	if !jsvalue.Truthy(expires) {
		expires = expiresDefault
	}

	return jsonwebtoken.Sign(
		[]jsonwebtoken.Field{
			{Key: "auth", Value: guid},
			{Key: "agent", Value: req.Header("user-agent")},
		},
		secret,
		jsonwebtoken.Options{ExpiresIn: expires},
	)
}

// GenerateAndStoreToken is generateAndStoreToken. The record is written with
// the same key order JSON.stringify produced, and the put error is discarded
// exactly as the original's empty callback discards it.
func GenerateAndStoreToken(req Request, opts ...TokenOptions) string {
	var options TokenOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	guid := generateGUID()

	token, err := generateToken(req, guid, options)
	if err != nil {
		panic(err)
	}

	record, err := json.Marshal(struct {
		Valid   bool  `json:"valid"`
		Created int64 `json:"created"`
	}{Valid: true, Created: time.Now().UnixMilli()})
	if err != nil {
		panic(err)
	}

	_ = db.Put(guid, string(record))

	return token
}

// Success is authSuccess.
func Success(req Request, res Response) Response {
	token := GenerateAndStoreToken(req)
	res.WriteHead(200, map[string]string{
		"content-type":  "text/html",
		"authorization": token,
	})

	return res.End(restricted)
}

var u = struct{ un, pw string }{un: "masterbuilder", pw: "itsnosecret"}

// Handler is authHandler. It returns any because the original returns two
// different things: the response object on the GET branch, and the request
// emitter that the .on() chain yields on the POST branch. The functional
// suite reads the status off the GET branch's return value.
func Handler(req Request, res Response) any {
	if req.Method() == "POST" {
		body := ""

		return req.OnData(func(data string) {
			body += data
		}).OnEnd(func() {
			post := qs.Parse(body)

			username, password := post["username"], post["password"]
			if jsvalue.Truthy(username) && eq(username, u.un) &&
				jsvalue.Truthy(password) && eq(password, u.pw) {
				Success(req, res)
				return
			}

			Fail(res)
		})
	}

	return Fail(res)
}

// eq is JavaScript's === for the parsed form values. A repeated key parses to
// an array, and an array is never strictly equal to a string, so the
// comparison has to fail rather than compare a joined rendering.
func eq(value any, want string) bool {
	got, ok := value.(string)
	return ok && got == want
}

// Verify returns the decoded claims, or false when the token is missing,
// malformed, expired or signed with another key.
func Verify(token any) any {
	var decoded any = false

	text, ok := token.(string)
	if !ok {
		return decoded
	}

	claims, err := jsonwebtoken.Verify(text, secret)
	if err != nil {
		return false
	}

	decoded = claims

	return decoded
}

func privado(res Response, token string) Response {
	res.WriteHead(200, map[string]string{
		"content-type":  "text/html",
		"authorization": token,
	})

	return res.End(restricted)
}

// Validate is validate. The callback runs on every path, including the two
// rejection paths, which is what lets the caller sequence its assertions.
func Validate(req Request, res Response, callback Callback) {
	token := req.Header("authorization")
	decoded := Verify(token)

	claims, ok := decoded.(jsonwebtoken.Claims)
	if !ok || !jsvalue.Truthy(claims["auth"]) {
		Fail(res)
		callback(res)

		return
	}

	record, err := db.Get(claims["auth"])

	// The original parses inside a try/catch and falls back to an invalid
	// record. Both arms are reachable: a missing key parses undefined, and a
	// logged out key parses the string level stored when handed an object.
	var parsed any
	if json.Unmarshal([]byte(record), &parsed) != nil {
		parsed = map[string]any{"valid": false}
	}

	if err != nil || !jsvalue.Truthy(property(parsed, "valid")) {
		Fail(res)
		callback(res)

		return
	}

	privado(res, jsvalue.String(token))
	callback(res)
}

// property reads a field off a parsed JSON value, reporting undefined for
// anything that is not an object, the way property access on a number or a
// string yields undefined rather than throwing.
func property(value any, name string) any {
	object, ok := value.(map[string]any)
	if !ok {
		return jsvalue.Undefined
	}

	found, ok := object[name]
	if !ok {
		return jsvalue.Undefined
	}

	return found
}

// Exit is exit. It really does terminate the process, through the swappable
// hook, because the server relies on that to shut itself down.
func Exit(res Response) {
	res.WriteHead(404, map[string]string{"content-type": "text/plain"})
	res.End("bye")
	ProcessExit(0)
}

// NotFound is notFound.
func NotFound(res Response) Response {
	res.WriteHead(404, map[string]string{"content-type": "text/plain"})
	return res.End("Not Found")
}

// Home is home.
func Home(res Response) Response {
	res.WriteHead(200, map[string]string{"content-type": "text/html"})
	return res.End(index)
}

// Done is done, the no-op continuation the router passes for routes that do
// not need to observe completion.
func Done(res Response) {}

// Logout is logout.
//
// The stored record is handed to Put as a map, not as JSON text. That is not
// an oversight: the original passes the mutated object straight through, and
// level's default utf8 encoding stringifies it to "[object Object]". The
// corrupted value is what later makes Validate reject the token, so writing
// well formed JSON here would silently restore access after logout.
func Logout(req Request, res Response, callback Callback) {
	token := req.Header("authorization")
	decoded := Verify(token)

	if !jsvalue.Truthy(decoded) {
		Fail(res, Done)
		callback(res)

		return
	}

	claims, _ := decoded.(jsonwebtoken.Claims)

	record, _ := db.Get(claims["auth"])

	// Unguarded on purpose: the original's JSON.parse here sits outside any
	// try/catch and throws for a record that is missing or already corrupted.
	updated := map[string]any{}
	if err := json.Unmarshal([]byte(record), &updated); err != nil {
		panic(err)
	}

	updated["valid"] = false

	_ = db.Put(claims["auth"], updated)

	res.WriteHead(200, map[string]string{"content-type": "text/plain"})
	res.End("Logged Out!")
	callback(res)
}
