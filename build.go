//usr/bin/env go run $0 $@; exit $?

//go:build ignore

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
		prodFlag = flag.Bool("prod", false, "Build in a production mode.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ./build.go [flags] [dir]\n")
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
		Src:  ".",
		Dst:  dir,
		Prod: *prodFlag,
	}

	if err := site.Build(c); err != nil {
		log.Fatal(err)
	}
}
