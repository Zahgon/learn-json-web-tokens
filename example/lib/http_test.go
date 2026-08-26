package lib

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dwyl/learn-json-web-tokens/internal/jsvalue"
)

func post(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(body))
}

func TestRequestMethod(t *testing.T) {
	if method := NewRequest(post("")).Method(); method != http.MethodPost {
		t.Errorf("Method() = %q, want %q", method, http.MethodPost)
	}
}

func TestRequestHeaderIsCaseInsensitive(t *testing.T) {
	request := post("")
	request.Header.Set("User-Agent", "Mozilla/5.0")

	adapted := NewRequest(request)

	for _, name := range []string{"user-agent", "User-Agent", "USER-AGENT"} {
		if value := adapted.Header(name); value != "Mozilla/5.0" {
			t.Errorf("Header(%q) = %v, want %q", name, value, "Mozilla/5.0")
		}
	}
}

func TestRequestHeaderOfAnAbsentHeaderIsUndefined(t *testing.T) {
	value := NewRequest(post("")).Header("authorization")

	if value != jsvalue.Undefined {
		t.Errorf("Header(absent) = %v, want undefined", value)
	}

	if jsvalue.Truthy(value) {
		t.Error("an absent header should not be truthy")
	}
}

func TestRequestHeaderOfAnEmptyHeaderIsAnEmptyString(t *testing.T) {
	request := post("")
	request.Header.Set("authorization", "")

	value := NewRequest(request).Header("authorization")

	if value != "" {
		t.Errorf("Header(empty) = %v, want an empty string", value)
	}

	if value == jsvalue.Undefined {
		t.Error("an empty header should be distinguishable from a missing one")
	}
}

func TestRequestListenersReceiveTheBodyInRegistrationOrder(t *testing.T) {
	request := NewRequest(post("username=masterbuilder"))

	var order []string

	chained := request.OnData(func(chunk string) {
		order = append(order, "first:"+chunk)
	})

	if chained != request {
		t.Error("OnData should return the request so listeners can be chained")
	}

	chained.OnData(func(chunk string) {
		order = append(order, "second:"+chunk)
	}).OnEnd(func() {
		order = append(order, "end")
	})

	request.Flush()

	want := []string{"first:username=masterbuilder", "second:username=masterbuilder", "end"}

	if len(order) != len(want) {
		t.Fatalf("dispatched %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Errorf("dispatch %d = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRequestFlushOfAnEmptyBodyStillEnds(t *testing.T) {
	request := NewRequest(post(""))

	data := 0
	ended := 0

	request.OnData(func(string) { data++ })
	request.OnEnd(func() { ended++ })

	request.Flush()

	if data != 0 {
		t.Errorf("data listeners ran %d times for an empty body, want 0", data)
	}

	if ended != 1 {
		t.Errorf("end listeners ran %d times, want 1", ended)
	}
}

func TestRequestFlushWithoutListenersDoesNothing(t *testing.T) {
	NewRequest(post("ignored")).Flush()
}

func TestResponseWriteHeadAndEnd(t *testing.T) {
	recorder := httptest.NewRecorder()
	response := NewResponse(recorder)

	returned := response.WriteHead(401, map[string]string{"content-type": "text/html"})

	if returned != response {
		t.Error("WriteHead should return the response so calls can be chained")
	}

	if ended := response.End("denied"); ended != response {
		t.Error("End should return the response")
	}

	result := recorder.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != 401 {
		t.Errorf("status = %d, want 401", result.StatusCode)
	}

	if got := result.Header.Get("Content-Type"); got != "text/html" {
		t.Errorf("Content-Type = %q, want %q", got, "text/html")
	}

	if got := result.Header.Get("Content-Length"); got != "6" {
		t.Errorf("Content-Length = %q, want %q", got, "6")
	}

	if body := recorder.Body.String(); body != "denied" {
		t.Errorf("body = %q, want %q", body, "denied")
	}
}

func TestResponseDefaultsToTwoHundred(t *testing.T) {
	recorder := httptest.NewRecorder()

	NewResponse(recorder).End("")

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Length"); got != "0" {
		t.Errorf("Content-Length = %q, want %q", got, "0")
	}
}

func TestResponseCarriesTheAuthorizationHeader(t *testing.T) {
	recorder := httptest.NewRecorder()

	NewResponse(recorder).
		WriteHead(200, map[string]string{"content-type": "text/html", "authorization": "a.b.c"}).
		End("restricted")

	if got := recorder.Header().Get("authorization"); got != "a.b.c" {
		t.Errorf("authorization = %q, want %q", got, "a.b.c")
	}
}
