// © 2026 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

import "../css/starplay.css"
import "../css/main.css"
import "../css/starfield.css"
import Lightense from "../vendor/lightense.min.js"
import { Starfield } from "./starfield.ts"
import { Starplay } from "./starplay.ts"

type Position = readonly [number, number]

interface EntryState {
  siteEntry?: number
  siteScroll?: Position
}

function enhance() {
  const images = document.querySelectorAll<HTMLImageElement>(
    "#page img:not(.avatar)",
  )
  if (images.length) Lightense(images, { background: "rgba(0, 0, 0, 0)" })
}

function scrollToPage(url: URL, position?: Position) {
  if (position) {
    scrollTo(...position)
    return
  }

  let id = url.hash.slice(1)
  try {
    id = decodeURIComponent(id)
  } catch {
    // Keep malformed fragments literal.
  }
  const target = id &&
    (document.getElementById(id) || document.getElementsByName(id)[0])
  if (target) target.scrollIntoView()
  else scrollTo(0, 0)
}

function samePage(a: URL, b: URL) {
  return a.pathname === b.pathname && a.search === b.search
}

function assets(doc: Document, url: URL) {
  return Array.from(
    doc.head.querySelectorAll('script[src], link[rel="stylesheet"]'),
    (el) =>
      new URL(el.getAttribute(el.tagName === "SCRIPT" ? "src" : "href")!, url)
        .href,
  ).join("\n")
}

class Navigation {
  #page: HTMLElement
  #current = new URL(location.href)
  #pending: AbortController | null = null
  #sequence = history.state?.siteEntry || 0
  #entry = this.#sequence
  #positions = new Map<number, Position>()

  constructor(page: HTMLElement) {
    this.#page = page
    history.scrollRestoration = "manual"

    document.addEventListener("click", (event) => this.onClick(event))
    addEventListener("pagehide", () => this.onPageHide())
    addEventListener("popstate", (event) => this.onPopState(event))
    addEventListener("hashchange", () => this.onHashChange())
    addEventListener("scroll", () => this.saveScroll(), { passive: true })

    const restored = this.selectEntry(history.state)
    if (restored) {
      const url = this.#current
      addEventListener("load", () => {
        if (location.href === url.href) scrollToPage(url, restored)
      }, { once: true })
    }
    this.saveScroll()
  }

  saveScroll() {
    // During Back/Forward, the address changes before the content does.
    if (location.href !== this.#current.href) return
    this.#positions.set(this.#entry, [scrollX, scrollY])
  }

  selectEntry(state: EntryState | null) {
    this.#entry = state?.siteEntry ?? ++this.#sequence
    this.#sequence = Math.max(this.#sequence, this.#entry)
    history.replaceState({ ...state, siteEntry: this.#entry }, "")
    return this.#positions.get(this.#entry) || state?.siteScroll
  }

  push(url: URL) {
    this.#entry = ++this.#sequence
    history.pushState({ siteEntry: this.#entry }, "", url)
  }

  cancel() {
    this.#pending?.abort()
    this.#pending = null
    this.#page.removeAttribute("aria-busy")
  }

  async visit(url: URL, position?: Position, pop = false) {
    this.cancel()
    const request = new AbortController()
    this.#pending = request
    this.#page.setAttribute("aria-busy", "true")

    try {
      const response = await fetch(url, {
        signal: request.signal,
        headers: { Accept: "text/html" },
      })
      if (
        !response.ok ||
        !response.headers.get("content-type")?.includes("text/html")
      ) throw new Error("not an HTML page")

      const destination = new URL(response.url || url.href)
      destination.hash = url.hash
      const next = new DOMParser().parseFromString(
        await response.text(),
        "text/html",
      )
      if (this.#pending !== request) return

      const content = next.getElementById("page")
      const main = content?.querySelector("main")
      // Keep full navigation for a new site version or an open image viewer.
      if (
        destination.origin !== location.origin ||
        !content || !main ||
        next.body.hasAttribute("data-reload") ||
        next.querySelector("base, #page script") ||
        document.querySelector(".lightense-open") ||
        assets(next, destination) !== assets(document, this.#current)
      ) throw new Error("full navigation required")

      if (pop) history.replaceState(history.state, "", destination)
      else this.push(destination)
      this.#current = destination
      document.title = next.title
      for (const el of document.head.querySelectorAll("meta[data-page]")) {
        el.remove()
      }
      document.head.append(...next.head.querySelectorAll("meta[data-page]"))

      for (const key of Object.keys(document.body.dataset)) {
        delete document.body.dataset[key]
      }
      Object.assign(document.body.dataset, next.body.dataset)
      this.#page.replaceChildren(...content.childNodes)
      enhance()
      main.tabIndex = -1
      main.focus({ preventScroll: true })
      scrollToPage(destination, position)
      document.dispatchEvent(new Event("site:load"))

      // Cached images may still be decoding when Back restores a long page.
      if (position || destination.hash) {
        await Promise.all(
          Array.from(
            this.#page.querySelectorAll("img"),
            (img) => img.decode().catch(() => {}),
          ),
        )
        if (this.#pending !== request) return
        scrollToPage(destination, position)
      }
      this.saveScroll()
    } catch {
      if (this.#pending === request) {
        if (pop) location.replace(url.href)
        else location.assign(url.href)
      }
    } finally {
      request.abort()
      if (this.#pending === request) {
        this.#pending = null
        this.#page.removeAttribute("aria-busy")
      }
    }
  }

  onClick(event: MouseEvent) {
    if (
      event.defaultPrevented || event.button !== 0 || event.metaKey ||
      event.ctrlKey || event.shiftKey || event.altKey
    ) return

    const link = event.target instanceof Element
      ? event.target.closest<HTMLAnchorElement>("a[href]")
      : null
    if (
      !link || link.isContentEditable || link.closest("[data-reload]") ||
      link.matches(
        '[download], [target]:not([target="_self"]), [rel~="external"]',
      )
    ) return

    const url = new URL(link.getAttribute("href")!, location.href)
    if (
      url.origin !== location.origin ||
      !["http:", "https:"].includes(url.protocol)
    ) return

    event.preventDefault()
    this.saveScroll()
    if (samePage(url, this.#current)) {
      this.cancel()
      if (url.href !== location.href) this.push(url)
      this.#current = url
      scrollToPage(url)
      this.saveScroll()
      return
    }
    this.visit(url)
  }

  onPageHide() {
    this.saveScroll()
    history.replaceState(
      { ...history.state, siteScroll: this.#positions.get(this.#entry) },
      "",
    )
    this.cancel()
  }

  onPopState(event: PopStateEvent) {
    const url = new URL(location.href)
    const position = this.selectEntry(event.state)
    if (samePage(url, this.#current)) {
      this.cancel()
      this.#current = url
      scrollToPage(url, position)
      return
    }
    this.visit(url, position, true)
  }

  onHashChange() {
    const url = new URL(location.href)
    if (!samePage(url, this.#current)) return
    this.cancel()
    this.#current = url
    this.selectEntry(history.state)
    this.saveScroll()
  }
}

new Starfield()
new Starplay()

const page = document.getElementById("page")
if (page) {
  enhance()
  if (!document.body.hasAttribute("data-reload")) new Navigation(page)
}
