(function () {
  "use strict";

  let ready = null;

  function report(error) {
    const output = document.getElementById("output");
    const button = document.getElementById("run");
    if (output) output.value = `Unable to load the playground: ${error}`;
    if (button) button.disabled = true;
  }

  async function load(url) {
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch(url),
      go.importObject,
    );
    go.run(instance).catch((error) => {
      ready = null;
      report(error);
    });
    if (typeof window.run !== "function") {
      throw new Error("Go runtime did not start");
    }
  }

  function start() {
    const url = document.body.dataset.starplayWasm;
    const button = document.getElementById("run");
    if (!url || !button) return;
    button.disabled = true;
    if (!ready) {
      ready = load(url).catch((error) => {
        ready = null;
        throw error;
      });
    }
    ready.then(() => {
      if (button.isConnected) button.disabled = false;
    }, report);
  }

  document.addEventListener("site:load", start);
  start();
})();
