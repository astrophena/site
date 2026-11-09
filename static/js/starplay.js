(function () {
  "use strict";

  class Starplay {
    #ready = null;

    constructor() {
      document.addEventListener("site:load", () => this.start());
      this.start();
    }

    start() {
      const url = document.body.dataset.starplayWasm;
      const button = document.getElementById("run");
      if (!url || !button) return;
      button.disabled = true;
      if (!this.#ready) {
        this.#ready = this.#load(url).catch((error) => {
          this.#ready = null;
          throw error;
        });
      }
      this.#ready.then(
        () => {
          if (button.isConnected) button.disabled = false;
        },
        (error) => this.#report(error),
      );
    }

    async #load(url) {
      const go = new Go();
      const { instance } = await WebAssembly.instantiateStreaming(
        fetch(url),
        go.importObject,
      );
      go.run(instance).catch((error) => {
        this.#ready = null;
        this.#report(error);
      });
      if (typeof window.run !== "function") {
        throw new Error("Go runtime did not start");
      }
    }

    #report(error) {
      const output = document.getElementById("output");
      const button = document.getElementById("run");
      if (output) output.value = `Unable to load the playground: ${error}`;
      if (button) button.disabled = true;
    }
  }

  new Starplay();
})();
