package lib

import (
	"io"
	"net/http"
	"strconv"

	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

// HTTPRequest adapts a net/http request to the Request interface.
//
// The data and end listeners are collected rather than invoked immediately so
// that the body arrives after the handler has finished registering, which is
// the ordering node's event loop provides.
type HTTPRequest struct {
	request *http.Request
	data    []func(chunk string)
	end     []func()
}

// NewRequest wraps an incoming request.
func NewRequest(request *http.Request) *HTTPRequest {
	return &HTTPRequest{request: request}
}

// Method reports the request method.
func (r *HTTPRequest) Method() string { return r.request.Method }

// Header reports a header value, or undefined when the header is absent, so
// that callers can tell an empty header from a missing one.
func (r *HTTPRequest) Header(name string) any {
	values, ok := r.request.Header[http.CanonicalHeaderKey(name)]
	if !ok || len(values) == 0 {
		return jsvalue.Undefined
	}

	return values[0]
}

// OnData registers a body chunk listener.
func (r *HTTPRequest) OnData(fn func(chunk string)) Request {
	r.data = append(r.data, fn)
	return r
}

// OnEnd registers a body completion listener.
func (r *HTTPRequest) OnEnd(fn func()) Request {
	r.end = append(r.end, fn)
	return r
}

// Flush reads the body and dispatches it to the registered listeners in
// registration order. The server calls it once the route has been chosen.
func (r *HTTPRequest) Flush() {
	if len(r.data) == 0 && len(r.end) == 0 {
		return
	}

	body, err := io.ReadAll(r.request.Body)
	if err == nil && len(body) > 0 {
		for _, fn := range r.data {
			fn(string(body))
		}
	}

	for _, fn := range r.end {
		fn()
	}
}

// HTTPResponse adapts a net/http response writer to the Response interface.
//
// The status line is held back until the body is known so that the response
// can be given a length and pushed out to the socket in one piece. The exit
// route ends the process the instant it has answered, so a response that is
// still sitting in a buffer when that happens would never reach the client.
type HTTPResponse struct {
	writer http.ResponseWriter
	status int
}

// NewResponse wraps an outgoing response.
func NewResponse(writer http.ResponseWriter) *HTTPResponse {
	return &HTTPResponse{writer: writer, status: http.StatusOK}
}

// WriteHead sets the status line and headers.
func (r *HTTPResponse) WriteHead(status int, headers map[string]string) Response {
	for name, value := range headers {
		r.writer.Header().Set(name, value)
	}

	r.status = status

	return r
}

// End writes the body and pushes it out to the client.
func (r *HTTPResponse) End(body string) Response {
	r.writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	r.writer.WriteHeader(r.status)

	_, _ = io.WriteString(r.writer, body)

	if flusher, ok := r.writer.(http.Flusher); ok {
		flusher.Flush()
	}

	return r
}
