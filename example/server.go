// Command server is the Go port of example/server.js. The routing stays here,
// in the executable, rather than moving into the library, because that is
// where the original keeps it.
package main

import (
	"fmt"
	"net/http"
	"os"

	app "github.com/dwyl/learn-json-web-tokens/example/lib"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "1337"
	}

	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			req, res := app.NewRequest(r), app.NewResponse(w)

			switch {
			case path == "/" || path == "/home":
				app.Home(res)
			case path == "/auth":
				app.Handler(req, res)
			case path == "/private":
				app.Validate(req, res, app.Done)
			case path == "/logout":
				app.Logout(req, res, app.Done)
			case path == "/exit":
				app.Exit(res)
			default:
				app.NotFound(res)
			}

			req.Flush()
		}),
	}

	fmt.Println("Visit: http://127.0.0.1:" + port)

	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
