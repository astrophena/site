// A quiet, decorative star atlas behind the homepage.
//
// The scene is a sparse, evenly spread field of faint stars plus a few real
// constellations, projected from their approximate sky coordinates and drawn
// as hairlines, like on an old star chart.
//
// It is rendered on demand: a frame is drawn only when something changes (a
// resize, a star slowly twinkling, a faint pulse travelling along a
// constellation, or mouse parallax settling). Between those moments nothing is
// drawn and the GPU stays idle. Animated frames are capped at 15–30 fps, and
// animation stops entirely when the page is hidden, after a while without user
// input, when the user prefers reduced motion, or when frames turn out to be
// slow. Without WebGL (or with a software-only implementation) the same scene
// is drawn once with Canvas 2D. Without JavaScript the page simply stays as
// it is.
(function () {
  "use strict";

  // Times are in milliseconds, distances in CSS pixels.
  const MAX_DPR = 2;
  const MAX_PIXELS = 3.5e6; // cap on the canvas backing store size
  const MAX_STARS = 420;
  const STAR_DENSITY = 8000; // CSS px² of viewport per background star
  const MIN_STARS = 60;
  const LINE_ALPHA = 0.16;
  const TWINKLE_GAP = [2500, 7000];
  const TWINKLE_LENGTH = [2000, 3500];
  const PULSE_GAP = [15000, 30000];
  const PULSE_SPEED = 70; // px per second
  const PARALLAX = 7; // maximum offset of the nearest stars
  const DORMANT_AFTER = 90000; // stop animating after this long without input
  const QUIET_DIM = 0.3; // brightness of stars behind text

  // A few real constellations. Stars are [right ascension (hours),
  // declination (degrees), magnitude, tint], where tint < 0 is bluish and
  // tint > 0 is warm. Lines are pairs of star indices. `wide` and `tall` place
  // the group on roomy landscape and portrait/compact viewports, respectively,
  // as [x, y, size, rotation]:
  // x and y are fractions of the viewport, size is a fraction of its shorter
  // side, and rotation is in degrees. Groups without a placement are hidden.
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
      lines: [0, 1, 1, 2, 2, 3, 4, 1, 1, 5, 5, 6, 7, 8, 8, 9, 9, 10, 10, 11, 11, 8],
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

  const reducedMotion = window.matchMedia
    ? window.matchMedia("(prefers-reduced-motion: reduce)")
    : { matches: false };

  const finePointer = window.matchMedia
    ? window.matchMedia("(hover: hover) and (pointer: fine)")
    : { matches: true };

  // Deterministic PRNG (mulberry32), so that the sky is the same on every
  // visit.
  function prng(seed) {
    return function () {
      seed = (seed + 0x6d2b79f5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }
  const rand = Math.random;
  const between = (r) => r[0] + rand() * (r[1] - r[0]);
  const smoothstep = (a, b, x) => {
    const t = Math.min(1, Math.max(0, (x - a) / (b - a)));
    return t * t * (3 - 2 * t);
  };

  // Project each constellation onto a plane around its center and normalize
  // it to fit [-1, 1].
  for (const c of CONSTELLATIONS) {
    let dec0 = 0;
    for (const s of c.stars) dec0 += s[1] / c.stars.length;
    const k = 15 * Math.cos((dec0 * Math.PI) / 180);
    // Right ascension grows eastwards, which is to the left on a sky chart.
    const pts = c.stars.map((s) => [-s[0] * k, -s[1]]);
    const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1]);
    const cx = (Math.min(...xs) + Math.max(...xs)) / 2;
    const cy = (Math.min(...ys) + Math.max(...ys)) / 2;
    const ext = Math.max(Math.max(...xs) - cx, Math.max(...ys) - cy);
    c.pts = pts.map((p) => [(p[0] - cx) / ext, (p[1] - cy) / ext]);
  }

  // Scene state, rebuilt by layout().
  let W = 0, H = 0, dpr = 1;
  let starData = new Float32Array(0); // x, y, id, size, brightness, depth, tint
  let lineData = new Float32Array(0); // x, y, depth
  let groups = []; // visible constellations, used for pulses
  const quietSelectors = ["header h1", "header nav", "main", "footer"];
  const quiet = new Float32Array(quietSelectors.length * 4); // x0, y0, x1, y1

  // Animation state.
  const twinkles = [null, null, null, null];
  const twinkleId = new Float32Array([-1, -1, -1, -1]);
  const twinkleAmp = new Float32Array(4);
  let pulse = null;
  const pulseU = new Float32Array(4); // x, y, intensity, depth
  const offset = [0, 0], target = [0, 0];
  let nextTwinkle = 0, nextPulse = 0, lastActivity = 0, lastFrame = 0;

  // Scheduling state.
  let canvas = null, renderer = null;
  let raf = 0, timer = 0, timerIsCap = false, expected = 0;
  let needsLayout = true, quietDirty = true, lost = false, degraded = false;
  let slowScore = 0, sampled = 0;

  function createCanvas() {
    if (canvas) canvas.remove();
    canvas = document.createElement("canvas");
    canvas.id = "starfield";
    canvas.setAttribute("aria-hidden", "true");
    document.body.prepend(canvas);
    needsLayout = true;
  }

  function resizeCanvas() {
    const w = canvas.clientWidth, h = canvas.clientHeight;
    dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR, Math.sqrt(MAX_PIXELS / Math.max(1, w * h)));
    const bw = Math.round(w * dpr), bh = Math.round(h * dpr);
    if (bw !== canvas.width || bh !== canvas.height) {
      canvas.width = bw;
      canvas.height = bh;
    }
    if (w === W && h === H) return false;
    W = w;
    H = h;
    return true;
  }

  function layout() {
    const stars = [], lines = [], taken = [];
    groups = [];

    const unit = Math.min(W, H), compact = unit < 600;
    const sizeScale = compact ? 1.15 : 1;
    for (const c of CONSTELLATIONS) {
      // Small screens get three larger, separated figures, including when
      // a phone is turned sideways. Keep their stars clear of the edges.
      const place = compact || W < H ? c.tall : c.wide;
      if (!place) continue;
      const half = Math.min(place[2] * unit, 420) / 2;
      const rot = (place[3] * Math.PI) / 180, cos = Math.cos(rot), sin = Math.sin(rot);
      const pts = c.pts.map((p) => [(p[0] * cos - p[1] * sin) * half, (p[0] * sin + p[1] * cos) * half]);
      const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1]);
      const cx = Math.max(16 - Math.min(...xs), Math.min(W - 16 - Math.max(...xs), place[0] * W));
      const cy = Math.max(16 - Math.min(...ys), Math.min(H - 16 - Math.max(...ys), place[1] * H));
      const depth = 0.5;
      const base = stars.length / 7, pos = [];
      pts.forEach((p, i) => {
        const x = cx + p[0], y = cy + p[1];
        const mag = c.stars[i][2];
        const size = (2.3 - 0.28 * mag) * sizeScale;
        stars.push(x, y, base + i, size, Math.min(0.85, Math.max(0.3, 0.85 - 0.12 * mag)), depth, c.stars[i][3]);
        pos.push([x, y, size]);
        taken.push([x, y]);
      });
      // Leave a small gap between lines and stars, like on a printed chart.
      for (let i = 0; i < c.lines.length; i += 2) {
        const a = pos[c.lines[i]], b = pos[c.lines[i + 1]];
        const dx = b[0] - a[0], dy = b[1] - a[1], len = Math.hypot(dx, dy);
        const ga = a[2] + 3, gb = b[2] + 3;
        if (len < ga + gb + 4) continue;
        lines.push(a[0] + (dx * ga) / len, a[1] + (dy * ga) / len, depth);
        lines.push(b[0] - (dx * gb) / len, b[1] - (dy * gb) / len, depth);
      }
      groups.push({ pos, lines: c.lines, depth });
    }

    // Best-candidate sampling in viewport pixels avoids both clumps and the
    // loss of stars from cropping a square sky to a narrow or wide screen.
    // Start from the same seed on every layout; cap retries on tiny screens.
    const r = prng(0x5ca1ab1e), sky = [];
    const want = Math.min(MAX_STARS, Math.max(MIN_STARS, Math.round((W * H) / STAR_DENSITY)));
    for (let attempt = 0; attempt < want * 2 && sky.length < want; attempt++) {
      let bx = 0, by = 0, best = -1;
      for (let c = 0; c < 4; c++) {
        const x = r() * W, y = r() * H;
        if (taken.some((p) => Math.abs(p[0] - x) < 16 && Math.abs(p[1] - y) < 16)) continue;
        let d = Infinity;
        for (const p of sky) {
          const dx = x - p[0], dy = y - p[1];
          d = Math.min(d, dx * dx + dy * dy);
        }
        if (d > best) {
          best = d;
          bx = x;
          by = y;
        }
      }
      if (best < 0) continue;
      const m = Math.pow(r(), 3); // most stars are faint
      const tint = r() < 0.15 ? 0.3 + r() * 0.5 : -r() * 0.6;
      stars.push(bx, by, stars.length / 7, (0.7 + 1.3 * m) * sizeScale, 0.18 + 0.46 * m, 0.15 + 0.6 * m + 0.25 * r(), tint);
      sky.push([bx, by]);
    }

    // The last vertex is the head of a travelling pulse, positioned by the
    // shader.
    stars.push(0, 0, -1, 2.2, 0, 0, -0.5);

    starData = new Float32Array(stars);
    lineData = new Float32Array(lines);
    pulse = null;
    pulseU[2] = 0;
    for (let i = 0; i < 4; i++) stopTwinkle(i);
  }

  // Keep each text region separate: a full-width footer must not dim the
  // gutters beside a long main column. Its range measures the text, not the
  // block's empty space. Clamp off-screen bounds for low-precision shaders.
  function measureQuiet() {
    quiet.fill(-9000);
    quietSelectors.forEach((selector, i) => {
      const el = document.querySelector(selector);
      if (!el) return;
      let r = el.getBoundingClientRect();
      if (selector === "footer") {
        const range = document.createRange();
        range.selectNodeContents(el);
        r = range.getBoundingClientRect();
      }
      if (!r.width || !r.height || r.bottom < -56 || r.top > H + 56) return;
      quiet.set([
        Math.max(-64, r.left), Math.max(-64, r.top),
        Math.min(W + 64, r.right), Math.min(H + 64, r.bottom),
      ], i * 4);
    });
  }

  function invalidateQuiet() {
    quietDirty = true;
    request();
  }

  function quietAt(x, y) {
    let k = 1;
    for (let i = 0; i < quiet.length; i += 4) {
      const d = Math.max(quiet[i] - x, x - quiet[i + 2], quiet[i + 1] - y, y - quiet[i + 3]);
      k = Math.min(k, smoothstep(-8, 56, d));
    }
    return QUIET_DIM + (1 - QUIET_DIM) * k;
  }

  // Animation.

  function canAnimate() {
    return renderer && renderer.animated && !reducedMotion.matches && !degraded;
  }

  function stopTwinkle(i) {
    twinkles[i] = null;
    twinkleId[i] = -1;
    twinkleAmp[i] = 0;
  }

  function startPulse(now) {
    if (!groups.length) return null;
    const g = groups[Math.floor(rand() * groups.length)];
    const adj = g.pos.map(() => []);
    for (let i = 0; i < g.lines.length; i += 2) {
      adj[g.lines[i]].push(g.lines[i + 1]);
      adj[g.lines[i + 1]].push(g.lines[i]);
    }
    // Start at a loose end of the figure when there is one, then wander
    // along the lines without revisiting stars.
    const ends = adj.map((a, i) => (a.length === 1 ? i : -1)).filter((i) => i >= 0);
    let cur = ends.length ? ends[Math.floor(rand() * ends.length)] : 0;
    const path = [cur];
    while (path.length < 5) {
      const next = adj[cur].filter((n) => path.indexOf(n) < 0);
      if (!next.length) break;
      cur = next[Math.floor(rand() * next.length)];
      path.push(cur);
    }
    const pts = path.map((i) => g.pos[i]), cum = [0];
    for (let i = 1; i < pts.length; i++) {
      cum.push(cum[i - 1] + Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]));
    }
    const len = cum[cum.length - 1];
    const length = Math.min(9000, Math.max(3000, (len / PULSE_SPEED) * 1000));
    return { pts, cum, len, start: now, length, depth: g.depth };
  }

  // Advances animations to `now` and returns the delay until the next frame
  // that is needed, or 0 if nothing is moving.
  function update(now) {
    const dt = Math.min(100, now - lastFrame);
    lastFrame = now;
    if (!canAnimate()) return 0;
    const awake = now - lastActivity < DORMANT_AFTER;
    let delay = 0;

    // Twinkles: a slow, smooth swell (or dip) in brightness of a few stars.
    if (awake && now >= nextTwinkle) {
      const i = twinkles.indexOf(null), n = starData.length / 7 - 1;
      if (i >= 0 && n > 0) {
        const id = Math.floor(rand() * n);
        twinkles[i] = { start: now, length: between(TWINKLE_LENGTH), amp: rand() < 0.8 ? 0.5 + rand() * 0.5 : -0.4 };
        twinkleId[i] = id;
      }
      nextTwinkle = now + between(TWINKLE_GAP);
    }
    for (let i = 0; i < 4; i++) {
      const t = twinkles[i];
      if (!t) continue;
      const u = (now - t.start) / t.length;
      if (u >= 1) {
        stopTwinkle(i);
        continue;
      }
      const s = Math.sin(Math.PI * u);
      twinkleAmp[i] = t.amp * s * s;
      delay = 1000 / 15;
    }

    // Pulse: a faint light travelling along a constellation's lines.
    if (!pulse && awake && now >= nextPulse) {
      pulse = startPulse(now);
      nextPulse = now + between(PULSE_GAP);
    }
    if (pulse) {
      const u = (now - pulse.start) / pulse.length;
      if (u >= 1) {
        pulse = null;
        pulseU[2] = 0;
      } else {
        const d = u * u * (3 - 2 * u) * pulse.len, cum = pulse.cum;
        let i = 1;
        while (i < cum.length - 1 && cum[i] < d) i++;
        const a = pulse.pts[i - 1], b = pulse.pts[i];
        const f = (d - cum[i - 1]) / Math.max(1e-3, cum[i] - cum[i - 1]);
        pulseU[0] = a[0] + (b[0] - a[0]) * f;
        pulseU[1] = a[1] + (b[1] - a[1]) * f;
        pulseU[2] = 0.6 * smoothstep(0, 0.15, u) * (1 - smoothstep(0.8, 1, u));
        pulseU[3] = pulse.depth;
        delay = 1000 / 30;
      }
    }

    // Parallax: ease towards the pointer.
    const k = 1 - Math.exp(-dt / 400);
    for (let i = 0; i < 2; i++) {
      const diff = target[i] - offset[i];
      if (Math.abs(diff) < 0.03) {
        offset[i] = target[i];
      } else {
        offset[i] += diff * k;
        delay = 1000 / 30;
      }
    }
    return delay;
  }

  function resetAnimation() {
    for (let i = 0; i < 4; i++) stopTwinkle(i);
    pulse = null;
    pulseU[2] = 0;
    offset[0] = offset[1] = target[0] = target[1] = 0;
  }

  // Scheduling.

  function request() {
    if (raf || (timer && timerIsCap) || lost || !renderer || document.hidden) return;
    clearTimeout(timer);
    timer = 0;
    raf = requestAnimationFrame(frame);
  }

  function cancel() {
    cancelAnimationFrame(raf);
    clearTimeout(timer);
    raf = timer = 0;
    expected = 0;
  }

  function frame(now) {
    raf = 0;
    if (needsLayout) {
      needsLayout = false;
      if (resizeCanvas() || !starData.length) layout();
      renderer.upload();
      quietDirty = true;
    }
    if (quietDirty) {
      quietDirty = false;
      measureQuiet();
    }

    // If capped frames keep arriving late, the device is struggling: settle
    // on a static scene.
    if (expected) {
      slowScore = slowScore * 0.92 + (now - expected > 45 ? 0.08 : 0);
      if (++sampled > 30 && slowScore > 0.5) {
        degraded = true;
        resetAnimation();
      }
      expected = 0;
    }

    const delay = update(now);
    renderer.draw();
    if (!canvas.className) canvas.className = "ready";

    if (delay) {
      timerIsCap = true;
      expected = performance.now() + delay;
      timer = setTimeout(fire, delay - (performance.now() - now));
    } else if (canAnimate() && now - lastActivity < DORMANT_AFTER) {
      timerIsCap = false;
      timer = setTimeout(fire, Math.max(16, Math.min(nextTwinkle, nextPulse) - performance.now()));
    }
  }

  function fire() {
    timer = 0;
    request();
  }

  function onActivity() {
    const now = performance.now();
    if (now - lastActivity >= DORMANT_AFTER) {
      nextTwinkle = now + between([500, 2500]);
      nextPulse = now + between([5000, 12000]);
    }
    lastActivity = now;
    // Only wakes up a dormant scene; otherwise a frame is already scheduled.
    if (!raf && !timer && canAnimate()) request();
  }

  // Renderers.

  // Shared by the star vertex shader and the line fragment shader, so stars,
  // pulse heads, and every point along a line obey the same text mask.
  const QUIET_GLSL = `
    uniform vec4 u_quiet[${quietSelectors.length}];

    float quietAt(vec2 p) {
      float k = 1.0;
      for (int i = 0; i < ${quietSelectors.length}; i++) {
        vec2 d = max(u_quiet[i].xy - p, p - u_quiet[i].zw);
        k = min(k, smoothstep(-8.0, 56.0, max(d.x, d.y)));
      }
      return mix(${QUIET_DIM.toFixed(2)}, 1.0, k);
    }`;

  const STAR_VS = `
    attribute vec3 a_pos; // x, y, id
    attribute vec4 a_star; // size, brightness, depth, tint
    uniform vec2 u_res;
    uniform vec2 u_off;
    uniform vec4 u_pulse;
    uniform vec4 u_twId;
    uniform vec4 u_twAmp;
    uniform float u_dpr;
    varying vec3 v_color;

    ${QUIET_GLSL}

    void main() {
      vec2 p = a_pos.xy;
      float size = a_star.x;
      float b = a_star.y;
      float depth = a_star.z;
      if (a_pos.z < -0.5) {
        p = u_pulse.xy;
        b = u_pulse.z * 0.7;
        depth = u_pulse.w;
      } else {
        float tw = dot(step(abs(u_twId - a_pos.z), vec4(0.25)), u_twAmp);
        vec2 dp = (p - u_pulse.xy) / 100.0;
        float glow = u_pulse.z * exp(-dot(dp, dp) * 50.0);
        b = b * (1.0 + tw) + glow * 0.8;
        size *= 1.0 + 0.3 * max(tw, 0.0) + glow;
      }
      p += u_off * depth;
      b *= quietAt(p);
      vec3 tint = a_star.w < 0.0 ? vec3(0.68, 0.79, 1.0) : vec3(1.0, 0.82, 0.62);
      v_color = mix(vec3(0.9, 0.93, 1.0), tint, abs(a_star.w)) * b;
      gl_Position = vec4(p / u_res * vec2(2.0, -2.0) + vec2(-1.0, 1.0), 0.0, 1.0);
      gl_PointSize = max(2.0, size * 4.0 * u_dpr);
    }`;

  // A sharp core with a faint halo.
  const STAR_FS = `
    precision mediump float;
    varying vec3 v_color;

    void main() {
      vec2 q = gl_PointCoord * 2.0 - 1.0;
      float d2 = dot(q, q);
      gl_FragColor = vec4(v_color * (exp(-d2 * 9.0) + 0.07 * max(0.0, 1.0 - d2)), 1.0);
    }`;

  const LINE_VS = `
    attribute vec3 a_pos; // x, y, depth
    uniform vec2 u_res;
    // Match the fragment uniform even on devices without fragment highp.
    uniform mediump vec2 u_off;
    varying vec2 v_pos;

    void main() {
      vec2 p = a_pos.xy + u_off * a_pos.z;
      v_pos = p;
      gl_Position = vec4(p / u_res * vec2(2.0, -2.0) + vec2(-1.0, 1.0), 0.0, 1.0);
    }`;

  const LINE_FS = `
    #ifdef GL_FRAGMENT_PRECISION_HIGH
    precision highp float;
    #else
    precision mediump float;
    #endif
    uniform vec4 u_pulse;
    uniform mediump vec2 u_off;
    varying vec2 v_pos;
    ${QUIET_GLSL}

    void main() {
      vec2 d = clamp((v_pos - u_pulse.xy - u_off * u_pulse.w) / 100.0, -2.0, 2.0);
      float glow = u_pulse.z * exp(-dot(d, d) * 16.0);
      vec3 c = vec3(0.6, 0.7, 0.86) * ${LINE_ALPHA.toFixed(2)} + vec3(0.7, 0.82, 1.0) * glow * 0.6;
      gl_FragColor = vec4(c * quietAt(v_pos), 1.0);
    }`;

  function initGL() {
    const gl = canvas.getContext("webgl", {
      alpha: false,
      antialias: false,
      depth: false,
      stencil: false,
      powerPreference: "low-power",
      // Prefer the static fallback over software rendering.
      failIfMajorPerformanceCaveat: true,
    });
    if (!gl) return null;

    function program(vs, fs, attribs, uniforms) {
      const p = gl.createProgram();
      for (const [type, src] of [[gl.VERTEX_SHADER, vs], [gl.FRAGMENT_SHADER, fs]]) {
        const s = gl.createShader(type);
        gl.shaderSource(s, src);
        gl.compileShader(s);
        gl.attachShader(p, s);
        gl.deleteShader(s);
      }
      attribs.forEach((name, i) => gl.bindAttribLocation(p, i, name));
      gl.linkProgram(p);
      if (!gl.getProgramParameter(p, gl.LINK_STATUS)) return null;
      const u = {};
      for (const name of uniforms) u[name] = gl.getUniformLocation(p, name);
      return { p, u };
    }

    let stars, lines, starBuf, lineBuf, starCount = 0, lineCount = 0, bg = [0, 0, 0];

    function init() {
      const common = ["u_res", "u_off", "u_pulse", "u_quiet"];
      stars = program(STAR_VS, STAR_FS, ["a_pos", "a_star"], common.concat(["u_twId", "u_twAmp", "u_dpr"]));
      lines = program(LINE_VS, LINE_FS, ["a_pos"], common);
      if (!stars || !lines) return false;
      starBuf = gl.createBuffer();
      lineBuf = gl.createBuffer();
      gl.enable(gl.BLEND);
      gl.blendFunc(gl.ONE, gl.ONE); // light adds up
      gl.enableVertexAttribArray(0);
      const m = getComputedStyle(document.body).backgroundColor.match(/[\d.]+/g);
      if (m) bg = m.slice(0, 3).map((v) => v / 255);
      return true;
    }

    function common(prog) {
      gl.useProgram(prog.p);
      gl.uniform2f(prog.u.u_res, W, H);
      gl.uniform2f(prog.u.u_off, offset[0], offset[1]);
      gl.uniform4fv(prog.u.u_pulse, pulseU);
      gl.uniform4fv(prog.u.u_quiet, quiet);
    }

    if (!init()) return null;

    canvas.addEventListener("webglcontextlost", (e) => {
      e.preventDefault();
      lost = true;
      cancel();
    });
    canvas.addEventListener("webglcontextrestored", () => {
      lost = false;
      if (!init()) return fallback();
      needsLayout = true;
      request();
    });

    return {
      animated: true,
      upload() {
        gl.bindBuffer(gl.ARRAY_BUFFER, starBuf);
        gl.bufferData(gl.ARRAY_BUFFER, starData, gl.STATIC_DRAW);
        gl.bindBuffer(gl.ARRAY_BUFFER, lineBuf);
        gl.bufferData(gl.ARRAY_BUFFER, lineData, gl.STATIC_DRAW);
        starCount = starData.length / 7;
        lineCount = lineData.length / 3;
      },
      draw() {
        gl.viewport(0, 0, canvas.width, canvas.height);
        gl.clearColor(bg[0], bg[1], bg[2], 1);
        gl.clear(gl.COLOR_BUFFER_BIT);

        if (lineCount) {
          common(lines);
          gl.bindBuffer(gl.ARRAY_BUFFER, lineBuf);
          gl.vertexAttribPointer(0, 3, gl.FLOAT, false, 12, 0);
          gl.drawArrays(gl.LINES, 0, lineCount);
        }

        common(stars);
        gl.uniform4fv(stars.u.u_twId, twinkleId);
        gl.uniform4fv(stars.u.u_twAmp, twinkleAmp);
        gl.uniform1f(stars.u.u_dpr, dpr);
        gl.bindBuffer(gl.ARRAY_BUFFER, starBuf);
        gl.enableVertexAttribArray(1);
        gl.vertexAttribPointer(0, 3, gl.FLOAT, false, 28, 0);
        gl.vertexAttribPointer(1, 4, gl.FLOAT, false, 28, 12);
        gl.drawArrays(gl.POINTS, 0, starCount);
        gl.disableVertexAttribArray(1);
      },
    };
  }

  // Static Canvas 2D version of the same scene.
  function init2D() {
    const ctx = canvas.getContext("2d");
    if (!ctx) return null;
    const rgba = (c, a) => `rgba(${c.map((v) => Math.round(v * 255)).join(",")},${a.toFixed(3)})`;
    return {
      animated: false,
      upload() {},
      draw() {
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.globalCompositeOperation = "lighter";
        ctx.lineWidth = 1 / dpr;
        for (let i = 0; i < lineData.length; i += 6) {
          const x0 = lineData[i], y0 = lineData[i + 1], x1 = lineData[i + 3], y1 = lineData[i + 4];
          const gradient = ctx.createLinearGradient(x0, y0, x1, y1);
          const steps = Math.max(1, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / 12));
          for (let j = 0; j <= steps; j++) {
            const t = j / steps;
            gradient.addColorStop(t, rgba([0.6, 0.7, 0.86], LINE_ALPHA * quietAt(x0 + (x1 - x0) * t, y0 + (y1 - y0) * t)));
          }
          ctx.strokeStyle = gradient;
          ctx.beginPath();
          ctx.moveTo(x0, y0);
          ctx.lineTo(x1, y1);
          ctx.stroke();
        }
        for (let i = 0; i < starData.length - 7; i += 7) {
          const x = starData[i], y = starData[i + 1], size = starData[i + 3];
          const b = starData[i + 4] * quietAt(x, y), t = starData[i + 6];
          const tint = t < 0 ? [0.68, 0.79, 1] : [1, 0.82, 0.62];
          const c = [0.9, 0.93, 1].map((v, j) => v + (tint[j] - v) * Math.abs(t));
          ctx.fillStyle = rgba(c, b * 0.12);
          ctx.beginPath();
          ctx.arc(x, y, size * 1.8, 0, 2 * Math.PI);
          ctx.fill();
          ctx.fillStyle = rgba(c, Math.min(1, b * 1.2));
          ctx.beginPath();
          ctx.arc(x, y, size * 0.55, 0, 2 * Math.PI);
          ctx.fill();
        }
      },
    };
  }

  function fallback() {
    cancel();
    createCanvas();
    renderer = init2D();
    if (!renderer) {
      canvas.remove();
      return;
    }
    request();
  }

  // Start.

  // Wakes the scene from scratch, e.g. after the page becomes visible again.
  function restart() {
    lastActivity = -DORMANT_AFTER;
    quietDirty = true;
    onActivity();
    request();
  }

  function moveTarget(x, y) {
    target[0] = x;
    target[1] = y;
    onActivity();
    if (canAnimate()) request();
  }

  function start() {
    lastActivity = performance.now();
    nextTwinkle = lastActivity + between([1500, 4000]);
    nextPulse = lastActivity + between([6000, 12000]);

    createCanvas();
    renderer = initGL();
    if (renderer) request();
    else fallback();

    addEventListener("resize", () => {
      needsLayout = true;
      request();
    });
    addEventListener("scroll", invalidateQuiet, { passive: true });
    // Disclosures on the package index move the footer without a scroll.
    // Observe content sizes even when the scene is static or dormant.
    if (window.ResizeObserver) {
      const observer = new ResizeObserver(invalidateQuiet);
      for (const selector of quietSelectors) {
        const el = document.querySelector(selector);
        if (el) observer.observe(el);
      }
    }
    document.addEventListener("toggle", invalidateQuiet, true);
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) cancel();
      else restart();
    });
    const onMotionChange = () => {
      resetAnimation();
      cancel();
      restart();
    };
    if (reducedMotion.addEventListener) reducedMotion.addEventListener("change", onMotionChange);
    else if (reducedMotion.addListener) reducedMotion.addListener(onMotionChange);

    for (const type of ["pointerdown", "keydown", "wheel", "touchstart"]) {
      addEventListener(type, onActivity, { passive: true });
    }
    addEventListener(
      "pointermove",
      (e) => {
        if (!finePointer.matches || e.pointerType !== "mouse" || !W || !H) return onActivity();
        moveTarget((0.5 - e.clientX / W) * 2 * PARALLAX, (0.5 - e.clientY / H) * 2 * PARALLAX);
      },
      { passive: true },
    );
    document.documentElement.addEventListener("mouseleave", () => moveTarget(0, 0));
  }

  // Start only after the page has loaded, so that the content always comes
  // first.
  if (document.readyState === "complete") start();
  else addEventListener("load", start);
})();
