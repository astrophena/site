//usr/bin/env go run $0 $@ ; exit "$?"

// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

//go:build ignore

// This program builds the site.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.astrophena.name/site"
)

func main() {
	log.SetFlags(0)

	var (
		dirFlag    = flag.String("dir", filepath.Join(".", "build"), "Directory where to put the built site.")
		envFlag    = flag.String("env", "dev", "Environment to build for.")
		serveFlag  = flag.Bool("serve", false, "Serve the site.")
		listenFlag = flag.String("listen", "localhost:3000", "Listen when serving the site on `host:port`.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Available flags:\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSee https://go.astrophena.name/site for other documentation.\n")
	}
	flag.Parse()

	// Check if we are executed from repo root.
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "build.go")); os.IsNotExist(err) {
		log.Fatal("Are you at repo root?")
	} else if err != nil {
		log.Fatal(err)
	}

	c := &site.Config{
		Env:  site.Env(*envFlag),
		Src:  ".",
		Dst:  *dirFlag,
		Logf: logf,
	}

	if *serveFlag {
		if err := site.Serve(c, *listenFlag); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := site.Build(c); err != nil {
		log.Fatal(err)
	}
	return
}

func logf(format string, args ...interface{}) { log.Printf("==> "+format, args...) }
