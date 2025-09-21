// © 2025 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"go.astrophena.name/base/cli"
	"go.astrophena.name/site/internal/devtools/internal"
)

func main() {
	cli.Main(new(app))
}

type app struct {
	quality int
}

func (a *app) Flags(fs *flag.FlagSet) {
	fs.IntVar(&a.quality, "quality", 90, "WebP quality.")
}

func (a *app) Run(ctx context.Context) error {
	internal.EnsureRoot()

	if _, err := exec.LookPath("magick"); err != nil {
		return errors.New("ImageMagick (magick command) not found")
	}

	if len(flag.Args()) != 1 {
		return errors.New("usage: go tool resizeicons <input_image_file>")
	}
	inputFile := flag.Args()[0]

	absInputFile, err := filepath.Abs(inputFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for input file: %w", err)
	}
	if _, err := os.Stat(absInputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file %s not found", absInputFile)
	}

	outputDir := filepath.Join("static", "icons")

	sizes := []int{179, 191, 35}

	for _, size := range sizes {
		sizeStr := strconv.Itoa(size)
		outputFile := filepath.Join(outputDir, fmt.Sprintf("%sx%s.webp", sizeStr, sizeStr))

		centerX := size / 2
		centerY := size / 2
		perimY := 0

		drawCircleArg := fmt.Sprintf("circle %d,%d %d,%d", centerX, centerY, centerX, perimY)

		args := []string{
			absInputFile,
			"-resize", sizeStr + "x" + sizeStr + "^",
			"-gravity", "North",
			"-extent", sizeStr + "x" + sizeStr,
			"(", "+clone", "-alpha", "transparent", "-fill", "white", "-draw", drawCircleArg, ")",
			"-compose", "CopyOpacity",
			"-composite",
			"-quality", strconv.Itoa(a.quality),
			outputFile,
		}

		cmd := exec.CommandContext(ctx, "magick", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to resize icon to %s: %w", sizeStr, err)
		}
	}

	return nil
}
