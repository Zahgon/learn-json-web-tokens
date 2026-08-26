// Package test holds the Go port of the example test suite.
//
// mock.go is the port of example/test/mock.js. The request and response are
// package level singletons that the functional suite shares across every
// case and never resets, exactly as the original module exports a single
// pair. That sharing is observable: handler registers a fresh listener pair
// on each call while the previous pair stays attached, so a later emit runs
// every handler that was ever registered, in registration order.
package test

import (
	"github.com/dwyl/learn-json-web-tokens/example/lib"
	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

// EventEmitter is the subset of node's EventEmitter the mocks rely on:
// listeners fire in the order they were added, and nothing ever removes them.
type EventEmitter struct {
	listeners map[string][]func(arg any)
}

// On appends a listener.
func (e *EventEmitter) On(event string, fn func(arg any)) {
	if e.listeners == nil {
		e.listeners = map[string][]func(arg any){}
	}

	e.listeners[event] = append(e.listeners[event], fn)
}

// Emit calls every listener registered for the event, oldest first. The slice
// is snapshotted so a listener that registers another listener does not
// change the current dispatch, matching node's behaviour.
func (e *EventEmitter) Emit(event string, arg any) {
	current := append([]func(arg any){}, e.listeners[event]...)
	for _, fn := range current {
		fn(arg)
	}
}

// ListenerCount reports how many listeners an event carries.
func (e *EventEmitter) ListenerCount(event string) int {
	return len(e.listeners[event])
}

// MockRequest stands in for the node request.
type MockRequest struct {
	EventEmitter

	Headers   map[string]any
	MethodStr string
}

// Method reports the mutable method field.
func (r *MockRequest) Method() string { return r.MethodStr }

// Header reads the headers object with the same exact key matching plain
// property access performs, reporting undefined for an absent header.
func (r *MockRequest) Header(name string) any {
	value, ok := r.Headers[name]
	if !ok {
		return jsvalue.Undefined
	}

	return value
}

// OnData registers a body chunk listener.
func (r *MockRequest) OnData(fn func(chunk string)) lib.Request {
	r.On("data", func(arg any) {
		chunk, _ := arg.(string)
		fn(chunk)
	})

	return r
}

// OnEnd registers a body completion listener.
func (r *MockRequest) OnEnd(fn func()) lib.Request {
	r.On("end", func(any) { fn() })

	return r
}

// MockResponse stands in for the node response, recording what was written.
type MockResponse struct {
	EventEmitter

	Headers map[string]string
	Status  int
	Body    string
}

// WriteHead replaces the recorded headers and status wholesale, the way the
// original's assignment does, and returns the response.
func (r *MockResponse) WriteHead(status int, headers map[string]string) lib.Response {
	r.Headers = headers
	r.Status = status

	return r
}

// End records the body and returns the response.
func (r *MockResponse) End(body string) lib.Response {
	r.Body = body

	return r
}

// Req and Res are the shared singletons the suite mutates in place.
var (
	Req = &MockRequest{
		Headers: map[string]any{
			"Content-Type": "text/html",
			"user-agent":   "Mozilla/5.0",
		},
		MethodStr: "POST",
	}

	Res = &MockResponse{}
)
