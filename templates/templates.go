// Package templates contains Go templates for https://astrophena.name.
//
// It's intended for use in https://github.com/astrophena/vanity.
package templates

import _ "embed"

// Layout is the https://astrophena.name page layout template.
//
//go:embed layout.html
var Layout []byte
