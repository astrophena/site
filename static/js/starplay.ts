// © 2026 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

import "go:wasm_exec"

declare class Go {
  importObject: WebAssembly.Imports
  run(instance: WebAssembly.Instance): Promise<void>
}

declare global {
  interface Window {
    run?: () => void
  }
}

export class Starplay {
  #ready: Promise<void> | null = null

  constructor() {
    document.addEventListener("site:load", () => this.start())
    this.start()
  }

  start() {
    const url = document.body.dataset.starplayWasm
    const button = document.querySelector<HTMLButtonElement>("#run")
    if (!url || !button) return

    button.disabled = true
    this.#ready ??= this.#load(url).catch((error) => {
      this.#ready = null
      throw error
    })

    this.#ready.then(
      () => {
        if (button.isConnected) button.disabled = false
      },
      (error) => this.#report(error),
    )
  }

  async #load(url: string) {
    const go = new Go()
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch(url),
      go.importObject,
    )
    go.run(instance).catch((error) => {
      this.#ready = null
      this.#report(error)
    })
    if (typeof window.run !== "function") {
      throw new Error("Go runtime did not start")
    }
  }

  #report(error: unknown) {
    const output = document.querySelector<HTMLTextAreaElement>("#output")
    const button = document.querySelector<HTMLButtonElement>("#run")
    if (output) output.value = `Unable to load the playground: ${error}`
    if (button) button.disabled = true
  }
}
