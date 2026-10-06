/**
 * Demo cursor engine shared by every product mock on the marketing site.
 *
 * A scene is a timeline: waypoints the cursor must reach by a given time,
 * presses, and class toggles that switch the mock into its next state (a
 * drawer open, a toast shown). The mock's own CSS turns those classes into
 * the real app's transitions, so each reaction moves exactly like the
 * dashboard component it mirrors.
 *
 * The motion itself is what makes it read as a hand and not a keyframe:
 * the cursor rests, then leaves late and lands on time on a minimum-jerk
 * velocity profile (the curve human reaching movements follow), along a
 * slight arc that alternates sides, with a hover beat before every press.
 */

export type Step = {
  // Milliseconds into the loop.
  at: number;
  // Waypoint the cursor arrives at by `at`, in percent of the root box
  // (or px when the scene says so).
  x?: number;
  y?: number;
  // Or an element inside the root; ox/oy pick the spot within its box
  // (fractions, default centre). Measured live, so it survives scaling.
  el?: string;
  ox?: number;
  oy?: number;
  // Reach this waypoint in a straight line (a drag) and use the whole
  // segment for the move instead of resting first.
  straight?: boolean;
  slow?: boolean;
  // Element that gets `.is-hover` while the cursor rests on this waypoint.
  hover?: string;
  // Press here: the pointer dips, a ripple fires, `target` gets `.is-pressed`.
  press?: boolean;
  target?: string;
  // Hold the button down from here (a drag) until a step with `up`.
  down?: boolean;
  up?: boolean;
  // State switches on the root (or on the `on` element), applied at `at`
  // and undone when the loop restarts.
  add?: string[];
  remove?: string[];
  on?: string;
};

export type Scene = {
  loop: number;
  steps: Step[];
  unit?: '%' | 'px';
};

// Minimum-jerk profile: 10u^3 - 15u^4 + 6u^5.
const jerk = (u: number) => u * u * u * (10 + u * (-15 + 6 * u));

const PRESS_MS = 110;
const PRESSED_MS = 160;
const HOVER_BEAT_MS = 140;

export type Player = { stop: () => void; restart: () => void };

const noop: Player = { stop() {}, restart() {} };

export function playCursor(root: HTMLElement, scene: Scene): Player {
  const cursorElement = root.querySelector<HTMLElement>('.demo-cursor');
  if (!cursorElement) return noop;
  const cursor = cursorElement;

  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    root.classList.add('is-static');
    return noop;
  }

  const steps = [...scene.steps].sort((a, b) => a.at - b.at);
  const points = steps.filter((s) => s.el || (s.x != null && s.y != null));
  const ripple = cursor.querySelector<HTMLElement>('.demo-ripple');

  // The stage is the app window the mock sits in (the shell marks itself
  // with data-stage); the cursor is fixed inside it, so it can cross the
  // sidebar and header to reach a dialog that covers the whole screen.
  const stage = root.closest<HTMLElement>('[data-stage]') ?? root;

  // Stage px for a waypoint, or null while its element is not on screen yet
  // (a close button inside a panel that has not mounted).
  const toPx = (p: Step): [number, number] | null => {
    const sr = stage.getBoundingClientRect();
    // Undo the scale the page applies to the whole window.
    const k = sr.width ? stage.offsetWidth / sr.width : 1;
    if (p.el) {
      const el = root.querySelector(p.el);
      if (!el) return null;
      const r = el.getBoundingClientRect();
      if (!r.width && !r.height) return null;
      return [
        (r.left - sr.left + r.width * (p.ox ?? 0.5)) * k,
        (r.top - sr.top + r.height * (p.oy ?? 0.5)) * k,
      ];
    }
    const rr = root.getBoundingClientRect();
    const ox = (rr.left - sr.left) * k;
    const oy = (rr.top - sr.top) * k;
    return scene.unit === 'px'
      ? [ox + (p.x ?? 0), oy + (p.y ?? 0)]
      : [ox + ((p.x ?? 0) / 100) * root.offsetWidth, oy + ((p.y ?? 0) / 100) * root.offsetHeight];
  };

  let last: [number, number] = [0, 0];

  // Segment k runs from points[k] to points[k+1]: rest, then move so the
  // landing happens on time with a beat to spare before any press.
  function positionAt(t: number): [number, number, number] {
    let k = 0;
    while (k + 1 < points.length && points[k + 1].at <= t) k++;
    const a = points[k];
    const b = points[k + 1];
    if (!a) return [0, 0, -1];
    const [ax, ay] = toPx(a) ?? last;
    if (!b) return [ax, ay, k];
    const pb = toPx(b);
    // Hold until the target exists; nothing to aim at yet.
    if (!pb) return [ax, ay, k];
    const [bx, by] = pb;
    const dist = Math.hypot(bx - ax, by - ay);
    const avail = b.at - a.at;
    const dur = b.slow
      ? avail - HOVER_BEAT_MS
      : Math.min(avail - HOVER_BEAT_MS, Math.max(420, 360 + dist * 1.1));
    if (dur <= 0) return t >= b.at ? [bx, by, k + 1] : [ax, ay, k];
    const start = b.at - HOVER_BEAT_MS - dur;
    if (t <= start) return [ax, ay, k];
    if (t >= start + dur) return [bx, by, k + 1];
    const u = jerk((t - start) / dur);
    // A gentle arc: control point off the chord, alternating sides.
    const side = k % 2 ? 1 : -1;
    const nx = dist ? -(by - ay) / dist : 0;
    const ny = dist ? (bx - ax) / dist : 0;
    const amp = b.straight ? 0 : Math.min(40, dist * 0.09) * side;
    const cx = (ax + bx) / 2 + nx * amp;
    const cy = (ay + by) / 2 + ny * amp;
    const w = 1 - u;
    return [w * w * ax + 2 * w * u * cx + u * u * bx, w * w * ay + 2 * w * u * cy + u * u * by, -1];
  }

  let raf = 0;
  let t0 = 0;
  let running = false;
  let fired = new Set<Step>();
  let hoverEl: Element | null = null;
  let hoverIdx = -1;
  const added: { el: Element; cls: string }[] = [];
  const removed: { el: Element; cls: string }[] = [];
  const timers: number[] = [];

  const resolve = (sel?: string) => (sel ? root.querySelector(sel) : root);

  function fire(s: Step) {
    if (s.down) {
      cursor.classList.add('is-down');
      if (ripple) {
        ripple.classList.remove('is-rip');
        void ripple.offsetWidth;
        ripple.classList.add('is-rip');
      }
    }
    if (s.up) cursor.classList.remove('is-down');
    if (s.press) {
      cursor.classList.add('is-down');
      timers.push(window.setTimeout(() => cursor.classList.remove('is-down'), PRESS_MS));
      if (ripple) {
        ripple.classList.remove('is-rip');
        void ripple.offsetWidth;
        ripple.classList.add('is-rip');
      }
      const target = s.target ? root.querySelector(s.target) : null;
      if (target) {
        target.classList.add('is-pressed');
        timers.push(window.setTimeout(() => target.classList.remove('is-pressed'), PRESSED_MS));
      }
    }
    const el = resolve(s.on);
    if (!el) return;
    // Book-keeping so the loop reset lands back on the markup's own state:
    // a class added then removed in one loop needs no undo at all.
    const drop = (list: { el: Element; cls: string }[], cls: string) => {
      const i = list.findIndex((x) => x.el === el && x.cls === cls);
      if (i >= 0) list.splice(i, 1);
      return i >= 0;
    };
    for (const c of s.add ?? []) {
      if (!drop(removed, c) && !el.classList.contains(c)) added.push({ el, cls: c });
      el.classList.add(c);
    }
    for (const c of s.remove ?? []) {
      if (!drop(added, c) && el.classList.contains(c)) removed.push({ el, cls: c });
      el.classList.remove(c);
    }
  }

  function reset() {
    for (const t of timers) clearTimeout(t);
    timers.length = 0;
    for (const { el, cls } of added) el.classList.remove(cls);
    for (const { el, cls } of removed) el.classList.add(cls);
    added.length = 0;
    removed.length = 0;
    hoverEl?.classList.remove('is-hover');
    hoverEl = null;
    hoverIdx = -1;
    cursor.classList.remove('is-down');
    ripple?.classList.remove('is-rip');
    fired = new Set();
  }

  function restartCssTimeline() {
    // Mocks that keep keyframe reactions key them on .is-playing, so the
    // remove-reflow-add restarts them in step with the cursor.
    root.classList.remove('is-playing');
    void root.offsetWidth;
    root.classList.add('is-playing');
  }

  function frame(now: number) {
    if (!running) return;
    let t = now - t0;
    if (t >= scene.loop) {
      reset();
      t0 = now;
      t = 0;
      restartCssTimeline();
    }
    for (const s of steps) {
      if (s.at <= t && !fired.has(s)) {
        fired.add(s);
        fire(s);
      }
    }
    const [x, y, atIdx] = positionAt(t);
    last = [x, y];
    cursor.style.transform = `translate3d(${x}px, ${y}px, 0)`;
    if (atIdx !== hoverIdx) {
      hoverEl?.classList.remove('is-hover');
      hoverEl = null;
      hoverIdx = atIdx;
      const sel = atIdx >= 0 ? points[atIdx]?.hover : undefined;
      if (sel) {
        hoverEl = root.querySelector(sel);
        hoverEl?.classList.add('is-hover');
      }
    }
    raf = requestAnimationFrame(frame);
  }

  function start() {
    if (running) return;
    running = true;
    reset();
    t0 = performance.now();
    restartCssTimeline();
    raf = requestAnimationFrame(frame);
  }

  function stop() {
    if (!running) return;
    running = false;
    cancelAnimationFrame(raf);
    reset();
    root.classList.remove('is-playing');
  }

  let onScreen = false;
  const sync = () => {
    if (onScreen && !document.hidden) start();
    else stop();
  };
  const io = new IntersectionObserver(
    (entries) => {
      onScreen = entries.some((e) => e.isIntersecting);
      sync();
    },
    { threshold: 0.2 },
  );
  io.observe(root);
  document.addEventListener('visibilitychange', sync);

  return {
    stop() {
      io.disconnect();
      document.removeEventListener('visibilitychange', sync);
      stop();
    },
    restart() {
      stop();
      sync();
    },
  };
}
