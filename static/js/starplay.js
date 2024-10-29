const go = new Go();
WebAssembly.instantiateStreaming(fetch("/wasm/starplay.wasm"), go.importObject).then((result) => {
  go.run(result.instance);
});
