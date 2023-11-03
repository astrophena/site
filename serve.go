//usr/bin/env go run $0 $@; exit $?

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
)

func main() {
	log.SetFlags(0)

	listenFlag := flag.String("listen", "localhost:3000", "Listen on `host:port`.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ./serve.go [flags] [dir]\n")
		fmt.Fprintf(os.Stderr, "Available flags:\n")
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
		Src: ".",
		Dst: dir,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := site.Serve(ctx, c, *listenFlag); err != nil {
		log.Fatal(err)
	}
}
