// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

package main

import (
	"context"
	"flag"
	"path/filepath"

	"go.astrophena.name/base/cli"
	"go.astrophena.name/site/internal/devtools/internal"
	"go.astrophena.name/site/internal/site"
)

func main() { cli.Main(new(app)) }

type app struct {
	listen string
}

func (a *app) Flags(fs *flag.FlagSet) {
	fs.StringVar(&a.listen, "listen", "localhost:3000", "Listen on `host:port`.")
}

func (a *app) Run(ctx context.Context) error {
	internal.EnsureRoot()

	dir := filepath.Join(".", "build")
	if len(flag.Args()) > 0 {
		dir = flag.Args()[0]
	}

	cfg := &site.Config{
		Src: ".",
		Dst: dir,
	}
	return site.Serve(ctx, cfg, a.listen)
}
