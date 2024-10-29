//usr/bin/env go run $0 $@; exit $?

// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"go.astrophena.name/site"
	"go.astrophena.name/site/vanity"
)

func main() {
	log.SetFlags(0)

	var (
		prodFlag     = flag.Bool("prod", false, "Build in a production mode.")
		skipStarplay = flag.Bool("skip-starplay", false, "Skip building Starlark playground WASM module.")
		vanityFlag   = flag.Bool("vanity", false, "Build vanity import site instead of main one.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ./build.go [flags] [dir]\n")
		fmt.Fprintf(os.Stderr, "Available flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	wd := try(os.Getwd())
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); errors.Is(err, fs.ErrNotExist) {
		log.Fatal("Are you at repo root?")
	} else if err != nil {
		log.Fatal(err)
	}

	dir := filepath.Join(".", "build")
	if len(flag.Args()) > 0 {
		dir = flag.Args()[0]
	}

	if *vanityFlag {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		must(vanity.Build(ctx, &vanity.Config{
			Dir:         dir,
			GitHubToken: os.Getenv("GITHUB_TOKEN"),
			ImportRoot:  "go.astrophena.name",
		}))

		return
	}

	if !*skipStarplay {
		build := exec.Command("go", "build", "-o", filepath.Join("static", "wasm", "starplay.wasm"), "./starplay")
		build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		build.Stderr = os.Stderr
		must(build.Run())
	}

	c := &site.Config{
		Src:  ".",
		Dst:  dir,
		Prod: *prodFlag,
	}
	must(site.Build(c))
}

func try[T any](val T, err error) T {
	must(err)
	return val
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
