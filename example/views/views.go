// Package views embeds the HTML pages the server renders so that they travel
// with a compiled binary, the way require-time fs.readFileSync bundles them
// into the running process in the original.
package views

import "embed"

// FS holds the HTML templates.
//
//go:embed *.html
var FS embed.FS
