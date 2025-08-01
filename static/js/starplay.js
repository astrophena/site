window.addEventListener("load", function() {
  const wasmURL = document.body.dataset.starplayWasm;
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch(wasmURL), go.importObject)
    .then((result) => {
      go.run(result.instance);
    })
    .catch((error) => {
      alert("Error loading or running WASM module: " + error);
    });
});
