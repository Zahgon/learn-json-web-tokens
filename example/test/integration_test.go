package test

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestIntegration runs the eight cases of example/test/integration.js against a
// real server started as a separate process, the way the original started
// node. The final case asks the server to terminate itself, which only works
// while it lives outside the process running the tests.
func TestIntegration(t *testing.T) {
	port := freePort(t)
	host := "http://127.0.0.1:" + strconv.Itoa(port)

	server := startServer(t, port)
	defer stopServer(server)

	awaitServer(t, port)

	var token string

	t.Run("Connect to localhost "+host, func(t *testing.T) {
		res, _ := call(t, request(t, "GET", host+"/", nil, nil))
		equalInt(t, res.StatusCode, 200, "Status 200")
	})

	t.Run("Attempt auth "+host+"/auth (incorrect username/password should fail)", func(t *testing.T) {
		res, _ := call(t, credentials(t, host, "lordbusiness", "kragle"))
		equalInt(t, res.StatusCode, 401, "Cannot authenticate (incorrect un/pw)")
	})

	t.Run("Attempt to access restricted content: "+host+"/private without supplying a valid token!", func(t *testing.T) {
		res, _ := call(t, request(t, "GET", host+"/private", nil, map[string]string{
			"authorization": "invalid",
		}))
		equalInt(t, res.StatusCode, 401, "Private content access denied!")
	})

	t.Run("Authenticate "+host+"/auth", func(t *testing.T) {
		res, _ := call(t, credentials(t, host, "masterbuilder", "itsnosecret"))
		token = res.Header.Get("authorization")
		equalInt(t, res.StatusCode, 200, "Authenticated")
	})

	t.Run("Access restricted content: "+host+"/private", func(t *testing.T) {
		res, _ := call(t, request(t, "GET", host+"/private", nil, map[string]string{
			"authorization": token,
		}))
		equalInt(t, res.StatusCode, 200, "Private content accessed")
	})

	t.Run("Log out "+host+"/logout", func(t *testing.T) {
		res, body := call(t, request(t, "GET", host+"/logout", nil, map[string]string{
			"authorization": token,
		}))
		equal(t, body, "Logged Out!", "Exit server")
		equalInt(t, res.StatusCode, 200, "End tests!")
	})

	t.Run("Attempt access using expired token (after logout)", func(t *testing.T) {
		res, _ := call(t, request(t, "GET", host+"/private", nil, map[string]string{
			"authorization": token,
		}))
		equalInt(t, res.StatusCode, 401, "Access Denied! (as expected)")
	})

	t.Run("EXIT "+host+"/exit", func(t *testing.T) {
		res, body := call(t, request(t, "GET", host+"/exit", nil, nil))
		equal(t, body, "bye", "Exit server")
		equalInt(t, res.StatusCode, 404, "End tests!")
	})
}

// credentials builds the form post the auth route expects, carrying the same
// headers the original client sent.
func credentials(t *testing.T, host, username, password string) *http.Request {
	t.Helper()

	form := "username=" + username + "&password=" + password

	return request(t, "POST", host+"/auth", strings.NewReader(form), map[string]string{
		"Content-Length": strconv.Itoa(len(form)),
		"Content-Type":   "application/x-www-form-urlencoded",
		"user-agent":     "Mozilla/5.0",
	})
}

func request(t *testing.T, method, url string, body io.Reader, headers map[string]string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("could not build the %s request to %s: %v", method, url, err)
	}

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	return req
}

func call(t *testing.T, req *http.Request) (*http.Response, string) {
	t.Helper()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("the request to %s failed: %v", req.URL, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("could not read the response from %s: %v", req.URL, err)
	}

	return res, string(body)
}

// startServer compiles the example server and launches it on the given port.
func startServer(t *testing.T, port int) *exec.Cmd {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "server")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	build := exec.Command(goTool(t), "build", "-o", binary, "github.com/dwyl/learn-json-web-tokens/example")
	build.Dir = repoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("could not build the server: %v\n%s", err, output)
	}

	server := exec.Command(binary)
	server.Dir = repoRoot(t)
	server.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	server.Stdout = os.Stdout
	server.Stderr = os.Stderr

	if err := server.Start(); err != nil {
		t.Fatalf("could not start the server: %v", err)
	}

	return server
}

// stopServer kills the server if the exit route has not already ended it.
func stopServer(server *exec.Cmd) {
	if server.Process == nil {
		return
	}

	_ = server.Process.Kill()
	_ = server.Wait()
}

// awaitServer waits for the port to start accepting connections, replacing the
// fixed pause the original took before it began making requests.
func awaitServer(t *testing.T, port int) {
	t.Helper()

	address := "127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(20 * time.Second)

	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = connection.Close()
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("the server never started listening on %s", address)
}

// freePort reserves a port from the operating system so that concurrent runs
// cannot collide the way a randomly chosen number can.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve a port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func goTool(t *testing.T) string {
	t.Helper()

	candidate := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		candidate += ".exe"
	}

	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	resolved, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("could not locate the go tool: %v", err)
	}

	return resolved
}

// repoRoot walks up from this file to the directory holding the module.
func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine the location of the test sources")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
