// © 2025 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

/*
Resize-icons resizes the site icons.

# Usage

	$ go tool resize-icons <input_image_file>

This tool resizes the provided input image to various sizes required
by the site, applies a circular mask, and saves them as WebP images
in the "static/icons" directory.

It requires ImageMagick (the "magick" command) to be installed and
available in the system's PATH.
*/
package main

import (
	_ "embed"

	"go.astrophena.name/base/cli"
)

//go:embed doc.go
var doc []byte

func init() {
	cli.SetDocComment(doc)
}
