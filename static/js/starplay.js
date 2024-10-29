const go = new Go();
WebAssembly.instantiateStreaming(fetch("/wasm/starplay.wasm"), go.importObject)
  .then((result) => {
    go.run(result.instance);
  })
  .catch((error) => {
    alert("Error loading or running WASM module: " + error);
  });
