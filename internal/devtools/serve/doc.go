// © 2025 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

/*
Serve serves the site for local development.

# Usage:

	$ go tool serve [flags] [dir]

Serve performs an initial build and serves the output from dir
(default "build"). It then watches for file changes in the "pages",
"static", and "templates" directories and automatically rebuilds
the site.
*/
package main

import (
	_ "embed"

	"go.astrophena.name/base/cli"
)

//go:embed doc.go
var doc []byte

func init() { cli.SetDocComment(doc) }
