(function () {
  "use strict";

  const page = document.getElementById("page");
  if (!page) return;

  function enhance() {
    Lightense("#page img:not(.avatar)", { background: "rgba(0, 0, 0, 0)" });
  }

  enhance();
  if (document.body.hasAttribute("data-reload")) return;

  let current = new URL(location.href);
  let pending = null;
  let sequence = history.state?.siteEntry || 0;
  let entry = sequence;
  const positions = new Map();
  const scripts = new Map(
    Array.from(
      document.head.querySelectorAll("script[data-page]"),
      (
        el,
      ) => [el.dataset.page, {
        src: new URL(el.getAttribute("src"), current).href,
        ready: Promise.resolve(),
      }],
    ),
  );
  history.scrollRestoration = "manual";

  function saveScroll() {
    // During Back/Forward, the address changes before the content does.
    if (location.href !== current.href) return;
    positions.set(entry, [scrollX, scrollY]);
  }

  function selectEntry(state) {
    entry = state?.siteEntry ?? ++sequence;
    sequence = Math.max(sequence, entry);
    history.replaceState({ ...state, siteEntry: entry }, "");
    return positions.get(entry) || state?.siteScroll;
  }

  function push(url) {
    entry = ++sequence;
    history.pushState({ siteEntry: entry }, "", url);
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

  function cancel() {
    if (pending) pending.abort();
    pending = null;
    page.removeAttribute("aria-busy");
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

  function loadScript(el, url) {
    const src = new URL(el.getAttribute("src"), url).href;
    const existing = scripts.get(el.dataset.page);
    if (existing) {
      if (existing.src !== src) throw new Error("page script changed");
      return existing.ready;
    }
    const ready = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.dataset.page = el.dataset.page;
      script.onload = resolve;
      script.onerror = () => reject(new Error(`loading ${src}`));
      document.head.append(script);
    });
    scripts.set(el.dataset.page, { src, ready });
    return ready;
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

  async function visit(url, position, pop = false) {
    cancel();
    const request = new AbortController();
    const styles = new Map();
    pending = request;
    page.setAttribute("aria-busy", "true");
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
      if (pending !== request) return;
      const content = next.getElementById("page");
      // Keep full navigation for a new site version or an open image viewer.
      if (
        destination.origin !== location.origin ||
        !content?.querySelector("main") ||
        next.body.hasAttribute("data-reload") ||
        next.querySelector("base, #page script") ||
        document.querySelector(".lightense-open") ||
        assets(next, destination) !== assets(document, current)
      ) throw new Error("full navigation required");

      const selector = "meta[data-page], link[data-page]";
      const previous = document.head.querySelectorAll(selector);
      const head = Array.from(next.head.querySelectorAll(selector));
      // Fetch styles without applying them to the current page. Activate them
      // only once every stylesheet is ready and the content can be swapped.
      await Promise.all(
        head.filter((el) => el.matches('link[rel="stylesheet"]')).map((el) => {
          styles.set(el, el.getAttribute("media"));
          el.media = "not all";
          return loadStyle(el, destination, request.signal);
        }),
      );
      if (pending !== request) return;

      for (const script of next.head.querySelectorAll("script[data-page]")) {
        await loadScript(script, destination);
        if (pending !== request) return;
      }
      if (pop) history.replaceState(history.state, "", destination);
      else push(destination);
      current = destination;
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
      page.replaceChildren(...content.childNodes);
      enhance();
      const main = page.querySelector("main");
      main.tabIndex = -1;
      main.focus({ preventScroll: true });
      scrollToPage(destination, position);
      document.dispatchEvent(new Event("site:load"));

      // Cached images may still be decoding when Back restores a long page.
      if (position || destination.hash) {
        await Promise.all(
          Array.from(
            page.querySelectorAll("img"),
            (img) => img.decode().catch(() => {}),
          ),
        );
        if (pending !== request) return;
        scrollToPage(destination, position);
      }
      saveScroll();
    } catch {
      if (pending === request) {
        if (pop) location.replace(url.href);
        else location.assign(url.href);
      }
    } finally {
      request.abort();
      for (const el of styles.keys()) el.remove();
      if (pending === request) {
        pending = null;
        page.removeAttribute("aria-busy");
      }
    }
  }

  document.addEventListener("click", (event) => {
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
    saveScroll();
    if (samePage(url, current)) {
      cancel();
      if (url.href !== location.href) push(url);
      current = url;
      scrollToPage(url);
      saveScroll();
      return;
    }
    visit(url);
  });

  addEventListener("pagehide", () => {
    saveScroll();
    history.replaceState(
      { ...history.state, siteScroll: positions.get(entry) },
      "",
    );
    cancel();
  });
  addEventListener("popstate", (event) => {
    const url = new URL(location.href);
    const position = selectEntry(event.state);
    if (samePage(url, current)) {
      cancel();
      current = url;
      scrollToPage(url, position);
      return;
    }
    visit(url, position, true);
  });
  addEventListener("hashchange", () => {
    const url = new URL(location.href);
    if (!samePage(url, current)) return;
    cancel();
    current = url;
    selectEntry(history.state);
    saveScroll();
  });
  addEventListener("scroll", saveScroll, { passive: true });
  const restored = selectEntry(history.state);
  if (restored) {
    const url = current;
    addEventListener("load", () => {
      if (location.href === url.href) scrollToPage(url, restored);
    }, { once: true });
  }
  saveScroll();
})();
