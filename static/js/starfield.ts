// © 2026 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

const MAX_PIXELS = 3.5e6
const MAX_STARS = 420
const QUIET_DIM = 0.3
const LINE_ALPHA = 0.16
const IDLE_AFTER = 90000
const textRegions = ["header h1", "header nav", "main", "footer"]

type Star = ReturnType<typeof star>
type Point = readonly [number, number]
type Placement = readonly [number, number, number, number]

interface Constellation {
  stars: [number, number, number, number][]
  lines: [number, number][]
  wide: Placement
  tall?: Placement
}

// Stars: right ascension (hours), declination (degrees), magnitude, tint.
// Viewport placements: x, y, size as fractions, then rotation in degrees.
const CONSTELLATIONS = ([
  {
    // Ursa Major (the Big Dipper).
    stars: [
      [11.062, 61.75, 1.8, 0.6], // Dubhe
      [11.031, 56.38, 2.4, -0.2], // Merak
      [11.897, 53.69, 2.4, -0.2], // Phecda
      [12.257, 57.03, 3.3, -0.2], // Megrez
      [12.9, 55.96, 1.8, -0.2], // Alioth
      [13.399, 54.93, 2.2, -0.2], // Mizar
      [13.792, 49.31, 1.9, -0.5], // Alkaid
    ],
    lines: [[0, 1], [1, 2], [2, 3], [3, 0], [3, 4], [4, 5], [5, 6]],
    wide: [0.83, 0.5, 0.34, 0],
    tall: [0.59, 0.38, 0.72, -16],
  },
  {
    // Cassiopeia.
    stars: [
      [0.153, 59.15, 2.3, 0.2], // Caph
      [0.675, 56.54, 2.2, 0.6], // Schedar
      [0.945, 60.72, 2.5, -0.5], // Navi
      [1.43, 60.24, 2.7, -0.2], // Ruchbah
      [1.907, 63.67, 3.4, -0.4], // Segin
    ],
    lines: [[0, 1], [1, 2], [2, 3], [3, 4]],
    wide: [0.09, 0.4, 0.17, -8],
  },
  {
    // Orion.
    stars: [
      [5.919, 7.41, 0.5, 1], // Betelgeuse
      [5.242, -8.2, 0.1, -0.7], // Rigel
      [5.419, 6.35, 1.6, -0.5], // Bellatrix
      [5.533, -0.3, 2.2, -0.4], // Mintaka
      [5.603, -1.2, 1.7, -0.5], // Alnilam
      [5.679, -1.94, 1.8, -0.5], // Alnitak
      [5.796, -9.67, 2.1, -0.5], // Saiph
      [5.585, 9.93, 3.4, -0.4], // Meissa
    ],
    lines: [[7, 0], [7, 2], [0, 5], [2, 3], [3, 4], [4, 5], [5, 6], [3, 1]],
    wide: [0.17, 0.76, 0.32, 8],
    tall: [0.68, 0.8, 0.46, 8],
  },
  {
    // Cygnus and Lyra.
    stars: [
      [20.69, 45.28, 1.3, 0], // Deneb
      [20.37, 40.26, 2.2, 0.2], // Sadr
      [19.938, 35.08, 3.9, 0.4], // η Cygni
      [19.512, 27.96, 3.1, 0.8], // Albireo
      [19.75, 45.13, 2.9, -0.3], // Fawaris
      [20.77, 33.97, 2.5, 0.4], // Aljanah
      [21.216, 30.23, 3.2, 0.3], // ζ Cygni
      [18.616, 38.78, 0, -0.6], // Vega
      [18.746, 37.61, 4.3, -0.2], // ζ Lyrae
      [18.908, 36.9, 4.3, 0.5], // δ Lyrae
      [18.982, 32.69, 3.2, -0.4], // Sulafat
      [18.835, 33.36, 3.5, -0.3], // Sheliak
    ],
    lines: [
      [0, 1],
      [1, 2],
      [2, 3],
      [4, 1],
      [1, 5],
      [5, 6],
      [7, 8],
      [8, 9],
      [9, 10],
      [10, 11],
      [11, 8],
    ],
    wide: [0.6, 0.78, 0.36, -20],
  },
  {
    // Corona Borealis.
    stars: [
      [15.549, 31.36, 4.1, -0.2], // θ
      [15.464, 29.11, 3.7, -0.2], // Nusakan
      [15.578, 26.71, 2.2, -0.3], // Alphecca
      [15.712, 26.3, 3.8, -0.2], // γ
      [15.826, 26.07, 4.6, 0.5], // δ
      [15.96, 26.88, 4.1, 0.6], // ε
      [16.024, 29.85, 5, -0.2], // ι
    ],
    lines: [[0, 1], [1, 2], [2, 3], [3, 4], [4, 5], [5, 6]],
    wide: [0.36, 0.58, 0.11, 0],
    tall: [0.22, 0.64, 0.27, 0],
  },
] satisfies Constellation[]).map((c) => {
  // Project around each figure's center, then normalize to [-1, 1].
  const declination = c.stars.reduce((sum, s) => sum + s[1], 0) /
    c.stars.length
  const k = 15 * Math.cos(declination * Math.PI / 180)
  const points = c.stars.map((s) => [-s[0] * k, -s[1]])
  const xs = points.map((p) => p[0]), ys = points.map((p) => p[1])
  const x = (Math.min(...xs) + Math.max(...xs)) / 2
  const y = (Math.min(...ys) + Math.max(...ys)) / 2
  const radius = Math.max(Math.max(...xs) - x, Math.max(...ys) - y)
  return {
    ...c,
    points: points.map<Point>(([px, py]) => [
      (px - x) / radius,
      (py - y) / radius,
    ]),
  }
})

function random(seed: number) {
  return function () {
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function star(
  x: number,
  y: number,
  size: number,
  brightness: number,
  tint: number,
) {
  const color = tint < 0 ? [0.68, 0.79, 1] : [1, 0.82, 0.62]
  const rgb = [0.9, 0.93, 1].map((v, i) =>
    Math.round(255 * (v + (color[i] - v) * Math.abs(tint)))
  )
  return { x, y, size, brightness, rgb: rgb.join(",") }
}

export class Starfield {
  #canvas = document.createElement("canvas")
  #sky = document.createElement("canvas")
  #ctx!: CanvasRenderingContext2D
  #base!: CanvasRenderingContext2D
  #motion!: MediaQueryList
  #width = 0
  #height = 0
  #scale = 1
  #stars: Star[] = []
  #lines: (readonly [number, number, number, number])[] = []
  #quiet: DOMRect[] = []
  #layoutDirty = true
  #paintDirty = true
  #frame = 0
  #timer = 0
  #twinkle: {
    star: Star
    start: number
  } | null = null
  #lastActivity = 0

  constructor() {
    const ctx = this.#canvas.getContext("2d", { alpha: false })
    const base = this.#sky.getContext("2d", { alpha: false })
    if (!ctx || !base) return

    this.#ctx = ctx
    this.#base = base

    this.#canvas.id = "starfield"
    this.#canvas.setAttribute("aria-hidden", "true")
    document.body.prepend(this.#canvas)

    this.#motion = matchMedia("(prefers-reduced-motion: reduce)")
    this.#lastActivity = performance.now()

    addEventListener("resize", () => this.onResize())
    addEventListener("scroll", () => this.invalidate(), { passive: true })
    for (
      const type of [
        "pointermove",
        "pointerdown",
        "keydown",
        "wheel",
        "touchstart",
      ]
    ) {
      addEventListener(type, () => this.activity(), { passive: true })
    }
    document.addEventListener("site:load", () => this.invalidate())
    document.addEventListener("toggle", () => this.invalidate(), true)
    document.addEventListener("load", () => this.invalidate(), true)
    document.addEventListener(
      "visibilitychange",
      () => this.onVisibilityChange(),
    )
    this.#motion.addEventListener("change", () => {
      this.stop()
      this.invalidate()
    })
    if (window.ResizeObserver) {
      new ResizeObserver(() => this.invalidate()).observe(
        document.getElementById("page")!,
      )
    }
    this.request()
  }

  layout() {
    this.#stars = []
    this.#lines = []

    const width = this.#width, height = this.#height
    const unit = Math.min(width, height)
    const compact = unit < 600
    const sizeScale = compact ? 1.15 : 1

    for (const c of CONSTELLATIONS) {
      const place = compact || width < height ? c.tall : c.wide
      if (!place) continue
      const half = Math.min(place[2] * unit, 420) / 2
      const angle = place[3] * Math.PI / 180
      const cos = Math.cos(angle), sin = Math.sin(angle)
      const points = c.points.map<Point>(([x, y]) => [
        (x * cos - y * sin) * half,
        (x * sin + y * cos) * half,
      ])
      const xs = points.map((p) => p[0]), ys = points.map((p) => p[1])
      const x = Math.max(
        16 - Math.min(...xs),
        Math.min(width - 16 - Math.max(...xs), place[0] * width),
      )
      const y = Math.max(
        16 - Math.min(...ys),
        Math.min(height - 16 - Math.max(...ys), place[1] * height),
      )
      const figure = points.map(([px, py], i) => {
        const [, , magnitude, tint] = c.stars[i]
        return star(
          x + px,
          y + py,
          (2.3 - 0.28 * magnitude) * sizeScale,
          Math.min(0.85, Math.max(0.3, 0.85 - 0.12 * magnitude)),
          tint,
        )
      })

      this.#stars.push(...figure)
      for (const [from, to] of c.lines) {
        const a = figure[from], b = figure[to]
        const dx = b.x - a.x, dy = b.y - a.y
        const length = Math.hypot(dx, dy)
        const start = a.size + 3, end = b.size + 3
        if (length < start + end + 4) continue
        this.#lines.push([
          a.x + dx * start / length,
          a.y + dy * start / length,
          b.x - dx * end / length,
          b.y - dy * end / length,
        ])
      }
    }

    // Choose the best of four candidates to spread background stars evenly.
    const background: Star[] = []
    const rand = random(0x5ca1ab1e)
    const count = Math.min(
      MAX_STARS,
      Math.max(60, Math.round(width * height / 8000)),
    )
    for (
      let attempt = 0;
      attempt < count * 2 && background.length < count;
      attempt++
    ) {
      let x = 0, y = 0, best = -1
      for (let i = 0; i < 4; i++) {
        const px = rand() * width, py = rand() * height
        if (
          this.#stars.some((s) =>
            Math.abs(s.x - px) < 16 && Math.abs(s.y - py) < 16
          )
        ) continue
        let distance = Infinity
        for (const s of background) {
          distance = Math.min(distance, (s.x - px) ** 2 + (s.y - py) ** 2)
          if (distance <= best) break
        }
        if (distance > best) {
          x = px
          y = py
          best = distance
        }
      }
      if (best < 0) continue
      const magnitude = rand() ** 3
      const tint = rand() < 0.15 ? 0.3 + rand() * 0.5 : -rand() * 0.6
      background.push(
        star(
          x,
          y,
          (0.7 + 1.3 * magnitude) * sizeScale,
          0.18 + 0.46 * magnitude,
          tint,
        ),
      )
    }
    this.#stars.push(...background)
  }

  measureText() {
    this.#quiet = []
    for (const selector of textRegions) {
      const el = document.querySelector(selector)
      if (!el) continue
      let bounds = el.getBoundingClientRect()
      if (selector === "footer") {
        const range = document.createRange()
        range.selectNodeContents(el)
        bounds = range.getBoundingClientRect()
      }
      if (bounds.width && bounds.height) this.#quiet.push(bounds)
    }
  }

  brightnessAt(x: number, y: number) {
    let brightness = 1
    for (const r of this.#quiet) {
      const distance = Math.max(
        r.left - x,
        x - r.right,
        r.top - y,
        y - r.bottom,
      )
      const t = Math.min(1, Math.max(0, (distance + 8) / 64))
      brightness = Math.min(brightness, t * t * (3 - 2 * t))
    }
    return QUIET_DIM + (1 - QUIET_DIM) * brightness
  }

  drawStar(ctx: CanvasRenderingContext2D, s: Star, gain = 1) {
    const brightness = s.brightness * this.brightnessAt(s.x, s.y) * gain
    ctx.fillStyle = `rgba(${s.rgb},${brightness * 0.12})`
    ctx.beginPath()
    ctx.arc(s.x, s.y, s.size * 1.8, 0, 2 * Math.PI)
    ctx.fill()
    ctx.fillStyle = `rgba(${s.rgb},${Math.min(1, brightness * 1.2)})`
    ctx.beginPath()
    ctx.arc(s.x, s.y, s.size * 0.55, 0, 2 * Math.PI)
    ctx.fill()
  }

  paint() {
    this.measureText()
    const base = this.#base
    base.globalCompositeOperation = "source-over"
    base.fillStyle = getComputedStyle(document.body).backgroundColor
    base.fillRect(0, 0, this.#width, this.#height)
    base.globalCompositeOperation = "lighter"
    base.lineWidth = 1 / this.#scale
    for (const [x0, y0, x1, y1] of this.#lines) {
      const gradient = base.createLinearGradient(x0, y0, x1, y1)
      const steps = Math.max(1, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / 12))
      for (let i = 0; i <= steps; i++) {
        const t = i / steps
        const alpha = LINE_ALPHA *
          this.brightnessAt(x0 + (x1 - x0) * t, y0 + (y1 - y0) * t)
        gradient.addColorStop(t, `rgba(153,179,219,${alpha})`)
      }
      base.strokeStyle = gradient
      base.beginPath()
      base.moveTo(x0, y0)
      base.lineTo(x1, y1)
      base.stroke()
    }
    for (const s of this.#stars) this.drawStar(base, s)
    this.#paintDirty = false
  }

  draw(now: number) {
    this.#frame = 0
    clearTimeout(this.#timer)
    this.#timer = 0
    const canvas = this.#canvas, sky = this.#sky, ctx = this.#ctx

    if (this.#layoutDirty) {
      const w = canvas.clientWidth, h = canvas.clientHeight
      if (w !== this.#width || h !== this.#height) {
        this.#width = w
        this.#height = h
        this.layout()
        this.#twinkle = null
      }
      this.#scale = Math.min(
        devicePixelRatio || 1,
        2,
        Math.sqrt(MAX_PIXELS / Math.max(1, this.#width * this.#height)),
      )
      const pixelsWide = Math.round(this.#width * this.#scale),
        pixelsHigh = Math.round(this.#height * this.#scale)
      if (canvas.width !== pixelsWide || canvas.height !== pixelsHigh) {
        canvas.width = sky.width = pixelsWide
        canvas.height = sky.height = pixelsHigh
      }
      this.#base.setTransform(this.#scale, 0, 0, this.#scale, 0, 0)
      this.#layoutDirty = false
      this.#paintDirty = true
    }

    const repaint = this.#paintDirty
    if (repaint) this.paint()
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.globalCompositeOperation = "source-over"
    if (repaint) {
      ctx.drawImage(sky, 0, 0)
      canvas.classList.add("ready")
    } else if (this.#twinkle) {
      // Restore only the twinkling star's pixels between atlas repaints.
      const s = this.#twinkle.star
      const radius = s.size * 1.8 * this.#scale + 1
      const x = Math.max(0, Math.floor(s.x * this.#scale - radius))
      const y = Math.max(0, Math.floor(s.y * this.#scale - radius))
      const width = Math.min(
        canvas.width,
        Math.ceil(s.x * this.#scale + radius),
      ) - x
      const height = Math.min(
        canvas.height,
        Math.ceil(s.y * this.#scale + radius),
      ) - y
      ctx.drawImage(sky, x, y, width, height, x, y, width, height)
    }

    ctx.setTransform(this.#scale, 0, 0, this.#scale, 0, 0)
    ctx.globalCompositeOperation = "lighter"
    if (this.#twinkle) {
      const phase = (now - this.#twinkle.start) / 2500
      if (phase >= 1) {
        this.#twinkle = null
      } else {
        this.drawStar(
          ctx,
          this.#twinkle.star,
          Math.sin(Math.PI * phase) ** 2 * 0.7,
        )
      }
    }

    if (this.#motion.matches) return
    if (this.#twinkle) {
      this.#timer = setTimeout(() => this.request(), 1000 / 15)
    } else if (now - this.#lastActivity < IDLE_AFTER) {
      this.#timer = setTimeout(() => {
        this.#timer = 0
        if (performance.now() - this.#lastActivity >= IDLE_AFTER) return
        this.#twinkle = {
          star: this.#stars[Math.floor(Math.random() * this.#stars.length)],
          start: performance.now(),
        }
        this.request()
      }, 3000 + Math.random() * 5000)
    }
  }

  request() {
    if (!this.#frame && !document.hidden) {
      this.#frame = requestAnimationFrame((now) => this.draw(now))
    }
  }

  invalidate() {
    this.#paintDirty = true
    this.request()
  }

  stop() {
    clearTimeout(this.#timer)
    cancelAnimationFrame(this.#frame)
    this.#timer = this.#frame = 0
    this.#twinkle = null
  }

  activity() {
    this.#lastActivity = performance.now()
    if (!this.#motion.matches && !this.#timer) this.request()
  }

  onResize() {
    this.#layoutDirty = true
    this.request()
  }

  onVisibilityChange() {
    this.stop()
    if (!document.hidden) {
      this.activity()
      this.invalidate()
    }
  }
}
