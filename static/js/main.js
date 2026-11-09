(function () {
  "use strict";

  function enhance() {
    const images = document.querySelectorAll("#page img:not(.avatar)");
    if (images.length) Lightense(images, { background: "rgba(0, 0, 0, 0)" });
  }

  function scrollToPage(url, position) {
    if (position) {
      scrollTo(...position);
      return;
    }
    let id = url.hash.slice(1);
    try {
      id = decodeURIComponent(id);
    } catch { /* Keep malformed fragments literal. */ }
    const target = id &&
      (document.getElementById(id) || document.getElementsByName(id)[0]);
    if (target) target.scrollIntoView();
    else scrollTo(0, 0);
  }

  function samePage(a, b) {
    return a.pathname === b.pathname && a.search === b.search;
  }

  function assets(doc, url) {
    return Array.from(
      doc.head.querySelectorAll(
        'script[src]:not([data-page]), link[rel="stylesheet"]:not([data-page])',
      ),
      (el) =>
        new URL(el.getAttribute(el.tagName === "SCRIPT" ? "src" : "href"), url)
          .href,
    ).join("\n");
  }

  function loadStyle(el, url, signal) {
    return new Promise((resolve, reject) => {
      function finish(error) {
        el.onload = el.onerror = null;
        signal.removeEventListener("abort", abort);
        if (error) reject(error);
        else resolve();
      }
      function abort() {
        finish(new Error("navigation cancelled"));
      }
      el.href = new URL(el.getAttribute("href"), url).href;
      el.onload = () => finish();
      el.onerror = () => finish(new Error(`loading ${el.href}`));
      signal.addEventListener("abort", abort, { once: true });
      if (signal.aborted) abort();
      else document.head.append(el);
    });
  }

  class Navigation {
    #page;
    #current = new URL(location.href);
    #pending = null;
    #sequence = history.state?.siteEntry || 0;
    #entry = this.#sequence;
    #positions = new Map();
    #scripts;

    constructor(page) {
      this.#page = page;
      this.#scripts = new Map(
        Array.from(
          document.head.querySelectorAll("script[data-page]"),
          (
            el,
          ) => [el.dataset.page, {
            src: new URL(el.getAttribute("src"), this.#current).href,
            ready: Promise.resolve(),
          }],
        ),
      );
      history.scrollRestoration = "manual";

      document.addEventListener("click", (event) => this.onClick(event));
      addEventListener("pagehide", () => this.onPageHide());
      addEventListener("popstate", (event) => this.onPopState(event));
      addEventListener("hashchange", () => this.onHashChange());
      addEventListener("scroll", () => this.saveScroll(), { passive: true });

      const restored = this.selectEntry(history.state);
      if (restored) {
        const url = this.#current;
        addEventListener("load", () => {
          if (location.href === url.href) scrollToPage(url, restored);
        }, { once: true });
      }
      this.saveScroll();
    }

    saveScroll() {
      // During Back/Forward, the address changes before the content does.
      if (location.href !== this.#current.href) return;
      this.#positions.set(this.#entry, [scrollX, scrollY]);
    }

    selectEntry(state) {
      this.#entry = state?.siteEntry ?? ++this.#sequence;
      this.#sequence = Math.max(this.#sequence, this.#entry);
      history.replaceState({ ...state, siteEntry: this.#entry }, "");
      return this.#positions.get(this.#entry) || state?.siteScroll;
    }

    push(url) {
      this.#entry = ++this.#sequence;
      history.pushState({ siteEntry: this.#entry }, "", url);
    }

    cancel() {
      if (this.#pending) this.#pending.abort();
      this.#pending = null;
      this.#page.removeAttribute("aria-busy");
    }

    loadScript(el, url) {
      const src = new URL(el.getAttribute("src"), url).href;
      const existing = this.#scripts.get(el.dataset.page);
      if (existing) {
        if (existing.src !== src) throw new Error("page script changed");
        return existing.ready;
      }
      const ready = new Promise((resolve, reject) => {
        const script = document.createElement("script");
        script.async = false;
        script.src = src;
        script.dataset.page = el.dataset.page;
        script.onload = resolve;
        script.onerror = () => reject(new Error(`loading ${src}`));
        document.head.append(script);
      });
      this.#scripts.set(el.dataset.page, { src, ready });
      return ready;
    }

    async visit(url, position, pop = false) {
      this.cancel();
      const request = new AbortController();
      const styles = new Map();
      this.#pending = request;
      this.#page.setAttribute("aria-busy", "true");
      try {
        const response = await fetch(url, {
          signal: request.signal,
          headers: { Accept: "text/html" },
        });
        if (
          !response.ok ||
          !response.headers.get("content-type")?.includes("text/html")
        ) throw new Error("not an HTML page");
        const destination = new URL(response.url || url.href);
        destination.hash = url.hash;
        const next = new DOMParser().parseFromString(
          await response.text(),
          "text/html",
        );
        if (this.#pending !== request) return;
        const content = next.getElementById("page");
        // Keep full navigation for a new site version or an open image viewer.
        if (
          destination.origin !== location.origin ||
          !content?.querySelector("main") ||
          next.body.hasAttribute("data-reload") ||
          next.querySelector("base, #page script") ||
          document.querySelector(".lightense-open") ||
          assets(next, destination) !== assets(document, this.#current)
        ) throw new Error("full navigation required");

        const selector = "meta[data-page], link[data-page]";
        const previous = document.head.querySelectorAll(selector);
        const head = Array.from(next.head.querySelectorAll(selector));
        // Fetch styles without applying them to the current page. Activate them
        // only once every stylesheet is ready and the content can be swapped.
        await Promise.all(
          head.filter((el) => el.matches('link[rel="stylesheet"]')).map(
            (el) => {
              styles.set(el, el.getAttribute("media"));
              el.media = "not all";
              return loadStyle(el, destination, request.signal);
            },
          ),
        );
        if (this.#pending !== request) return;

        await Promise.all(
          Array.from(
            next.head.querySelectorAll("script[data-page]"),
            (script) => this.loadScript(script, destination),
          ),
        );
        if (this.#pending !== request) return;
        if (pop) history.replaceState(history.state, "", destination);
        else this.push(destination);
        this.#current = destination;
        document.title = next.title;
        for (const el of previous) el.remove();
        const before = document.head.querySelector(
          'link[rel="stylesheet"]:not([data-page])',
        );
        for (const el of head) {
          if (styles.has(el)) {
            const media = styles.get(el);
            if (media === null) el.removeAttribute("media");
            else el.setAttribute("media", media);
          }
          document.head.insertBefore(el, before);
        }
        styles.clear();
        for (const key of Object.keys(document.body.dataset)) {
          delete document.body.dataset[key];
        }
        Object.assign(document.body.dataset, next.body.dataset);
        this.#page.replaceChildren(...content.childNodes);
        enhance();
        const main = this.#page.querySelector("main");
        main.tabIndex = -1;
        main.focus({ preventScroll: true });
        scrollToPage(destination, position);
        document.dispatchEvent(new Event("site:load"));

        // Cached images may still be decoding when Back restores a long page.
        if (position || destination.hash) {
          await Promise.all(
            Array.from(
              this.#page.querySelectorAll("img"),
              (img) => img.decode().catch(() => {}),
            ),
          );
          if (this.#pending !== request) return;
          scrollToPage(destination, position);
        }
        this.saveScroll();
      } catch {
        if (this.#pending === request) {
          if (pop) location.replace(url.href);
          else location.assign(url.href);
        }
      } finally {
        request.abort();
        for (const el of styles.keys()) el.remove();
        if (this.#pending === request) {
          this.#pending = null;
          this.#page.removeAttribute("aria-busy");
        }
      }
    }

    onClick(event) {
      if (
        event.defaultPrevented || event.button !== 0 || event.metaKey ||
        event.ctrlKey || event.shiftKey || event.altKey
      ) return;
      const link = event.target.closest?.("a[href]");
      if (
        !link || link.isContentEditable || link.closest("[data-reload]") ||
        link.matches(
          '[download], [target]:not([target="_self"]), [rel~="external"]',
        )
      ) return;
      const url = new URL(link.getAttribute("href"), location.href);
      if (
        url.origin !== location.origin ||
        !["http:", "https:"].includes(url.protocol)
      ) return;

      event.preventDefault();
      this.saveScroll();
      if (samePage(url, this.#current)) {
        this.cancel();
        if (url.href !== location.href) this.push(url);
        this.#current = url;
        scrollToPage(url);
        this.saveScroll();
        return;
      }
      this.visit(url);
    }

    onPageHide() {
      this.saveScroll();
      history.replaceState(
        { ...history.state, siteScroll: this.#positions.get(this.#entry) },
        "",
      );
      this.cancel();
    }

    onPopState(event) {
      const url = new URL(location.href);
      const position = this.selectEntry(event.state);
      if (samePage(url, this.#current)) {
        this.cancel();
        this.#current = url;
        scrollToPage(url, position);
        return;
      }
      this.visit(url, position, true);
    }

    onHashChange() {
      const url = new URL(location.href);
      if (!samePage(url, this.#current)) return;
      this.cancel();
      this.#current = url;
      this.selectEntry(history.state);
      this.saveScroll();
    }
  }

  const page = document.getElementById("page");
  if (!page) return;
  enhance();
  if (!document.body.hasAttribute("data-reload")) new Navigation(page);
})();
