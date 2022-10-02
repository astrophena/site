//usr/bin/env go run $0 $@; exit $?

//go:build ignore

// This program serves the site.
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
)

func main() {
	log.SetFlags(0)

	var (
		envFlag    = flag.String("env", "dev", "Environment to build for.")
		listenFlag = flag.String("listen", "localhost:3000", "Listen on `host:port`.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: script/server.go [flags] [dir]\n")
		fmt.Fprintf(os.Stderr, "Available flags:\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Check if we are executed from the script directory.
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

	c := &site.Config{
		Env:  site.Env(*envFlag),
		Src:  ".",
		Dst:  dir,
		Logf: site.ColoredLogf,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := site.Serve(ctx, c, *listenFlag); err != nil {
		log.Fatal(err)
	}
}
