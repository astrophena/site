// © 2024 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

//go:build js

// Starplay implements the WebAssembly module that powers https://astrophena.name/starplay.
package main

import (
	"bytes"
	"fmt"
	"syscall/js"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func run() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		input := js.Global().Get("document").Call("getElementById", "input")
		output := js.Global().Get("document").Call("getElementById", "output")

		script := input.Get("value").String()

		var buf bytes.Buffer
		thread := &starlark.Thread{
			Print: func(_ *starlark.Thread, msg string) { fmt.Fprintln(&buf, msg) },
		}

		if _, err := starlark.ExecFileOptions(
			&syntax.FileOptions{
				While:           true,
				TopLevelControl: true,
				GlobalReassign:  true,
			},
			thread,
			"code.star",
			script,
			nil,
		); err != nil {
			js.Global().Call("alert", err.Error())
			output.Set("value", "")
			return nil
		}

		output.Set("value", buf.String())
		return nil
	})
}

func main() {
	js.Global().Set("run", run())
	<-make(chan struct{})
}
