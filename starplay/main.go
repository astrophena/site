//go:build js

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
		if len(args) != 1 {
			return "no arguments passed"
		}

		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return "unable to get document object"
		}
		outputArea := doc.Call("getElementById", "output")
		if !outputArea.Truthy() {
			return "unable to get output text area"
		}

		script := args[0].String()

		var buf bytes.Buffer
		thread := &starlark.Thread{
			Print: func(_ *starlark.Thread, msg string) { fmt.Fprintln(&buf, msg) },
		}

		if _, err := starlark.ExecFileOptions(
			&syntax.FileOptions{},
			thread,
			"code.star",
			script,
			nil,
		); err != nil {
			outputArea.Set("value", err.Error())
			return nil
		}

		outputArea.Set("value", buf.String())
		return nil
	})
}

func main() {
	js.Global().Set("run", run())
	<-make(chan struct{})
}
