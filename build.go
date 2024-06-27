//usr/bin/env go run $0 $@; exit $?

// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"go.astrophena.name/site"
	"go.astrophena.name/site/vanity"
)

func main() {
	log.SetFlags(0)

	var (
		prodFlag   = flag.Bool("prod", false, "Build in a production mode.")
		vanityFlag = flag.Bool("vanity", false, "Build vanity import site instead of main one.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ./build.go [flags] [dir]\n")
		fmt.Fprintf(os.Stderr, "Available flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); os.IsNotExist(err) {
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

		if err := vanity.Build(ctx, &vanity.Config{
			Dir:         dir,
			GitHubToken: os.Getenv("GITHUB_TOKEN"),
			ImportRoot:  "go.astrophena.name",
		}); err != nil {
			log.Fatal(err)
		}

		return
	}

	c := &site.Config{
		Src:  ".",
		Dst:  dir,
		Prod: *prodFlag,
	}

	if err := site.Build(c); err != nil {
		log.Fatal(err)
	}
}
