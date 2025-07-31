// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.astrophena.name/base/cli"
	"go.astrophena.name/site/internal/devtools/internal"
	"go.astrophena.name/site/internal/site"
	"go.astrophena.name/site/internal/vanity"
)

func main() { cli.Main(new(app)) }

type app struct {
	prod         bool
	skipStarplay bool
	vanity       bool
}

func (a *app) Flags(fs *flag.FlagSet) {
	fs.BoolVar(&a.prod, "prod", false, "Build in a production mode.")
	fs.BoolVar(&a.skipStarplay, "skip-starplay", false, "Skip building Starlark playground WASM module.")
	fs.BoolVar(&a.vanity, "vanity", false, "Build vanity import site instead of main one.")
}

func (a *app) Run(ctx context.Context) error {
	internal.EnsureRoot()

	dir := filepath.Join(".", "build")
	if len(flag.Args()) > 0 {
		dir = flag.Args()[0]
	}

	if a.vanity {
		return vanity.Build(ctx, &vanity.Config{
			Dir:         dir,
			GitHubToken: os.Getenv("GITHUB_TOKEN"),
			ImportRoot:  "go.astrophena.name",
		})
	}

	if !a.skipStarplay {
		gorootb, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			return err
		}
		goroot := strings.TrimSuffix(string(gorootb), "\n")

		wasmExecJS, err := os.ReadFile(filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"))
		if err != nil {
			return err
		}

		// Copy wasm_exec.js from GOROOT to prevent version incompatibility.
		if err := os.WriteFile(filepath.Join("static", "js", "go_wasm_exec.js"), wasmExecJS, 0o644); err != nil {
			return err
		}

		build := exec.Command(
			"go",
			"build",
			"-ldflags", "-s -w -buildid=",
			"-trimpath",
			"-o", filepath.Join("static", "wasm", "starplay.wasm"),
			"./internal/starplay",
		)
		build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return err
		}
	}

	c := &site.Config{
		Src:  ".",
		Dst:  dir,
		Prod: a.prod,
	}
	return site.Build(c)
}
