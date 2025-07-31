// © 2025 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

/*
Build builds the site.

# Usage

	$ go tool build [flags] [dir]

Builds the site into the specified directory dir. If dir is not provided,
it defaults to build in the current working directory.
*/
package main

import (
	_ "embed"

	"go.astrophena.name/base/cli"
)

//go:embed doc.go
var doc []byte

func init() { cli.SetDocComment(doc) }
