(function () {
  "use strict";

  const MAX_PIXELS = 3.5e6;
  const MAX_STARS = 420;
  const QUIET_DIM = 0.3;
  const LINE_ALPHA = 0.16;
  const IDLE_AFTER = 90000;
  const textRegions = ["header h1", "header nav", "main", "footer"];

  // Stars: right ascension (hours), declination (degrees), magnitude, tint.
  // Placements: x, y, size as fractions of the viewport, then rotation.
  // Compact screens use three figures, including in landscape orientation.
  const CONSTELLATIONS = [
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
      lines: [0, 1, 1, 2, 2, 3, 3, 0, 3, 4, 4, 5, 5, 6],
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
      lines: [0, 1, 1, 2, 2, 3, 3, 4],
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
      lines: [7, 0, 7, 2, 0, 5, 2, 3, 3, 4, 4, 5, 5, 6, 3, 1],
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
        0,
        1,
        1,
        2,
        2,
        3,
        4,
        1,
        1,
        5,
        5,
        6,
        7,
        8,
        8,
        9,
        9,
        10,
        10,
        11,
        11,
        8,
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
      lines: [0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6],
      wide: [0.36, 0.58, 0.11, 0],
      tall: [0.22, 0.64, 0.27, 0],
    },
  ];

  function random(seed) {
    return function () {
      seed = (seed + 0x6d2b79f5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  function star(x, y, size, brightness, tint) {
    const color = tint < 0 ? [0.68, 0.79, 1] : [1, 0.82, 0.62];
    const rgb = [0.9, 0.93, 1].map((v, i) =>
      Math.round(255 * (v + (color[i] - v) * Math.abs(tint)))
    );
    return { x, y, size, brightness, rgb: rgb.join(",") };
  }

  // Project around each figure's center, then normalize to [-1, 1].
  for (const c of CONSTELLATIONS) {
    const declination = c.stars.reduce((sum, s) => sum + s[1], 0) /
      c.stars.length;
    const k = 15 * Math.cos(declination * Math.PI / 180);
    const points = c.stars.map((s) => [-s[0] * k, -s[1]]);
    const xs = points.map((p) => p[0]), ys = points.map((p) => p[1]);
    const x = (Math.min(...xs) + Math.max(...xs)) / 2;
    const y = (Math.min(...ys) + Math.max(...ys)) / 2;
    const radius = Math.max(Math.max(...xs) - x, Math.max(...ys) - y);
    c.points = points.map((p) => [(p[0] - x) / radius, (p[1] - y) / radius]);
  }

  const canvas = document.createElement("canvas");
  const sky = document.createElement("canvas");
  const ctx = canvas.getContext("2d", { alpha: false });
  const base = sky.getContext("2d", { alpha: false });
  if (!ctx || !base) return;

  canvas.id = "starfield";
  canvas.setAttribute("aria-hidden", "true");
  document.body.prepend(canvas);

  const motion = matchMedia("(prefers-reduced-motion: reduce)");
  let width = 0, height = 0, scale = 1;
  let stars = [], lines = [], quiet = [];
  let layoutDirty = true, paintDirty = true;
  let frame = 0, timer = 0, twinkle = null;
  let lastActivity = performance.now();

  function layout() {
    stars = [];
    lines = [];
    const unit = Math.min(width, height), compact = unit < 600;
    const sizeScale = compact ? 1.15 : 1;
    for (const c of CONSTELLATIONS) {
      const place = compact || width < height ? c.tall : c.wide;
      if (!place) continue;
      const half = Math.min(place[2] * unit, 420) / 2;
      const angle = place[3] * Math.PI / 180;
      const cos = Math.cos(angle), sin = Math.sin(angle);
      const points = c.points.map((
        [x, y],
      ) => [(x * cos - y * sin) * half, (x * sin + y * cos) * half]);
      const xs = points.map((p) => p[0]), ys = points.map((p) => p[1]);
      const x = Math.max(
        16 - Math.min(...xs),
        Math.min(width - 16 - Math.max(...xs), place[0] * width),
      );
      const y = Math.max(
        16 - Math.min(...ys),
        Math.min(height - 16 - Math.max(...ys), place[1] * height),
      );
      const figure = points.map((p, i) =>
        star(
          x + p[0],
          y + p[1],
          (2.3 - 0.28 * c.stars[i][2]) * sizeScale,
          Math.min(0.85, Math.max(0.3, 0.85 - 0.12 * c.stars[i][2])),
          c.stars[i][3],
        )
      );
      stars.push(...figure);
      for (let i = 0; i < c.lines.length; i += 2) {
        const a = figure[c.lines[i]], b = figure[c.lines[i + 1]];
        const dx = b.x - a.x, dy = b.y - a.y, length = Math.hypot(dx, dy);
        const start = a.size + 3, end = b.size + 3;
        if (length < start + end + 4) continue;
        lines.push([
          a.x + dx * start / length,
          a.y + dy * start / length,
          b.x - dx * end / length,
          b.y - dy * end / length,
        ]);
      }
    }

    // Pick the best of four candidates to avoid clumps. Work in viewport
    // pixels so narrow screens don't lose stars by cropping a square sky.
    const taken = stars.slice(), background = [], rand = random(0x5ca1ab1e);
    const count = Math.min(
      MAX_STARS,
      Math.max(60, Math.round(width * height / 8000)),
    );
    for (
      let attempt = 0;
      attempt < count * 2 && background.length < count;
      attempt++
    ) {
      let x = 0, y = 0, best = -1;
      for (let i = 0; i < 4; i++) {
        const px = rand() * width, py = rand() * height;
        if (
          taken.some((s) => Math.abs(s.x - px) < 16 && Math.abs(s.y - py) < 16)
        ) continue;
        let distance = Infinity;
        for (const s of background) {
          distance = Math.min(distance, (s.x - px) ** 2 + (s.y - py) ** 2);
        }
        if (distance > best) {
          x = px;
          y = py;
          best = distance;
        }
      }
      if (best < 0) continue;
      const magnitude = rand() ** 3;
      const tint = rand() < 0.15 ? 0.3 + rand() * 0.5 : -rand() * 0.6;
      background.push(
        star(
          x,
          y,
          (0.7 + 1.3 * magnitude) * sizeScale,
          0.18 + 0.46 * magnitude,
          tint,
        ),
      );
    }
    stars.push(...background);
  }

  function measureText() {
    quiet = [];
    for (const selector of textRegions) {
      const el = document.querySelector(selector);
      if (!el) continue;
      let bounds = el.getBoundingClientRect();
      if (selector === "footer") {
        const range = document.createRange();
        range.selectNodeContents(el);
        bounds = range.getBoundingClientRect();
      }
      if (bounds.width && bounds.height) quiet.push(bounds);
    }
  }

  function brightnessAt(x, y) {
    let brightness = 1;
    for (const r of quiet) {
      const distance = Math.max(
        r.left - x,
        x - r.right,
        r.top - y,
        y - r.bottom,
      );
      const t = Math.min(1, Math.max(0, (distance + 8) / 64));
      brightness = Math.min(brightness, t * t * (3 - 2 * t));
    }
    return QUIET_DIM + (1 - QUIET_DIM) * brightness;
  }

  function drawStar(context, s, gain = 1) {
    const brightness = s.brightness * brightnessAt(s.x, s.y) * gain;
    context.fillStyle = `rgba(${s.rgb},${brightness * 0.12})`;
    context.beginPath();
    context.arc(s.x, s.y, s.size * 1.8, 0, 2 * Math.PI);
    context.fill();
    context.fillStyle = `rgba(${s.rgb},${Math.min(1, brightness * 1.2)})`;
    context.beginPath();
    context.arc(s.x, s.y, s.size * 0.55, 0, 2 * Math.PI);
    context.fill();
  }

  function paint() {
    measureText();
    base.globalCompositeOperation = "source-over";
    base.fillStyle = getComputedStyle(document.body).backgroundColor;
    base.fillRect(0, 0, width, height);
    base.globalCompositeOperation = "lighter";
    base.lineWidth = 1 / scale;
    for (const [x0, y0, x1, y1] of lines) {
      const gradient = base.createLinearGradient(x0, y0, x1, y1);
      const steps = Math.max(1, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / 12));
      for (let i = 0; i <= steps; i++) {
        const t = i / steps;
        const alpha = LINE_ALPHA *
          brightnessAt(x0 + (x1 - x0) * t, y0 + (y1 - y0) * t);
        gradient.addColorStop(t, `rgba(153,179,219,${alpha})`);
      }
      base.strokeStyle = gradient;
      base.beginPath();
      base.moveTo(x0, y0);
      base.lineTo(x1, y1);
      base.stroke();
    }
    for (const s of stars) drawStar(base, s);
    paintDirty = false;
  }

  function draw(now) {
    frame = 0;
    clearTimeout(timer);
    timer = 0;
    if (layoutDirty) {
      const w = canvas.clientWidth, h = canvas.clientHeight;
      if (w !== width || h !== height) {
        width = w;
        height = h;
        layout();
        twinkle = null;
      }
      scale = Math.min(
        devicePixelRatio || 1,
        2,
        Math.sqrt(MAX_PIXELS / Math.max(1, width * height)),
      );
      const pixelsWide = Math.round(width * scale),
        pixelsHigh = Math.round(height * scale);
      if (canvas.width !== pixelsWide || canvas.height !== pixelsHigh) {
        canvas.width = sky.width = pixelsWide;
        canvas.height = sky.height = pixelsHigh;
      }
      base.setTransform(scale, 0, 0, scale, 0, 0);
      layoutDirty = false;
      paintDirty = true;
    }
    if (paintDirty) paint();
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.globalCompositeOperation = "source-over";
    ctx.drawImage(sky, 0, 0);
    ctx.setTransform(scale, 0, 0, scale, 0, 0);
    ctx.globalCompositeOperation = "lighter";
    if (twinkle) {
      const phase = (now - twinkle.start) / 2500;
      if (phase >= 1) twinkle = null;
      else drawStar(ctx, twinkle.star, Math.sin(Math.PI * phase) ** 2 * 0.7);
    }
    canvas.classList.add("ready");
    if (motion.matches) return;
    if (twinkle) {
      timer = setTimeout(request, 1000 / 15);
    } else if (now - lastActivity < IDLE_AFTER) {
      timer = setTimeout(() => {
        timer = 0;
        if (performance.now() - lastActivity >= IDLE_AFTER) return;
        twinkle = {
          star: stars[Math.floor(Math.random() * stars.length)],
          start: performance.now(),
        };
        request();
      }, 3000 + Math.random() * 5000);
    }
  }

  function request() {
    if (!frame && !document.hidden) frame = requestAnimationFrame(draw);
  }

  function invalidate() {
    paintDirty = true;
    request();
  }

  function stop() {
    clearTimeout(timer);
    cancelAnimationFrame(frame);
    timer = frame = 0;
    twinkle = null;
  }

  function activity() {
    lastActivity = performance.now();
    if (!motion.matches && !timer) request();
  }

  addEventListener("resize", () => {
    layoutDirty = true;
    request();
  });
  addEventListener("scroll", invalidate, { passive: true });
  for (
    const type of [
      "pointermove",
      "pointerdown",
      "keydown",
      "wheel",
      "touchstart",
    ]
  ) {
    addEventListener(type, activity, { passive: true });
  }
  document.addEventListener("site:load", invalidate);
  document.addEventListener("toggle", invalidate, true);
  document.addEventListener("load", invalidate, true);
  document.addEventListener("visibilitychange", () => {
    stop();
    if (!document.hidden) {
      activity();
      invalidate();
    }
  });
  motion.addEventListener("change", () => {
    stop();
    invalidate();
  });
  if (window.ResizeObserver) {
    new ResizeObserver(invalidate).observe(document.getElementById("page"));
  }
  request();
})();
