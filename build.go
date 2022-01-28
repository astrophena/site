//usr/bin/env go run $0 $@ ; exit "$?"

//go:build ignore

// This program builds the site.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"go.astrophena.name/site"
)

func main() {
	log.SetFlags(0)

	var (
		dirFlag   = flag.String("dir", filepath.Join(".", "build"), "Directory where to put the built site.")
		envFlag   = flag.String("env", "dev", "Environment to build for.")
		serveFlag = flag.String("serve", "", "Serve the site on `host:port`.")
	)
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

	if *serveFlag != "" {
		if err := site.Serve(c, *serveFlag); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := site.Build(c); err != nil {
		log.Fatal(err)
	}
	return
}

func logf(format string, args ...any) { log.Printf("==> "+format, args...) }
