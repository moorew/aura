// Deterministic verification of the dismissibleSheet action — exercises the
// REAL shipped code (compiled from src/lib/actions/sheet.ts via esbuild) through
// realistic touch sequences, asserting scroll and dismiss behave correctly.
import { build } from 'esbuild';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const entry = resolve(here, '../src/lib/actions/sheet.ts');

const out = await build({
  entryPoints: [entry],
  bundle: true,
  format: 'esm',
  write: false,
  platform: 'neutral',
});
const code = out.outputFiles[0].text;
const mod = await import('data:text/javascript,' + encodeURIComponent(code));
const { dismissibleSheet } = mod;

// ── Minimal DOM mock ──────────────────────────────────────────────────────
class El {
  constructor(attrs = {}) {
    this.style = {};
    this.attrs = attrs;
    this._listeners = {};
    this.scrollTop = 0;
    this._child = null;
  }
  matches(sel) {
    return (sel.split(',').map(s => s.trim())).some(s => this.attrs._tag === s);
  }
  closest(sel) {
    if (sel === '[data-sheet-handle]') return this.attrs.handle ? this : null;
    return null;
  }
  querySelector() { return this._child; }
  addEventListener(type, fn) { (this._listeners[type] ||= []).push(fn); }
  removeEventListener(type, fn) {
    this._listeners[type] = (this._listeners[type] || []).filter(f => f !== fn);
  }
  fire(type, ev) { (this._listeners[type] || []).forEach(fn => fn(ev)); }
}

globalThis.HTMLElement = El;
globalThis.Element = El;
globalThis.document = { activeElement: null };

function touch(node, target, x, y) {
  return { touches: [{ clientX: x, clientY: y }], target, preventDefault() { this._pd = true; } };
}

function gesture(node, target, points) {
  // points: [{x,y}, ...] first = start
  let prevented = false;
  const start = points[0];
  node.fire('touchstart', touch(node, target, start.x, start.y));
  for (let i = 1; i < points.length; i++) {
    const ev = touch(node, target, points[i].x, points[i].y);
    node.fire('touchmove', ev);
    if (ev._pd) prevented = true;
  }
  node.fire('touchend', {});
  return prevented;
}

// ── Test runner ───────────────────────────────────────────────────────────
let pass = 0, fail = 0;
function check(name, cond) {
  if (cond) { pass++; console.log(`  ✓ ${name}`); }
  else { fail++; console.log(`  ✗ ${name}`); }
}

function makeSheet({ scrollTop = 0 } = {}) {
  const node = new El();
  const scroll = new El({ _tag: '[data-sheet-scroll]' });
  scroll.scrollTop = scrollTop;
  node._child = scroll;
  let closed = false;
  const action = dismissibleSheet(node, {
    onClose: () => { closed = true; },
    scrollSelector: '[data-sheet-scroll]',
    threshold: 110,
  });
  return { node, scroll, action, isClosed: () => closed, content: new El({ _tag: 'div' }), handle: new El({ handle: true }) };
}

console.log('dismissibleSheet behaviour:');

// A) content scrolled down, swipe DOWN → must NOT dismiss, must NOT hijack (no preventDefault)
{
  const s = makeSheet({ scrollTop: 200 });
  const prevented = gesture(s.node, s.content, [{ x: 100, y: 100 }, { x: 100, y: 160 }, { x: 100, y: 260 }]);
  check('scrolled content: swipe down does NOT dismiss', !s.isClosed());
  check('scrolled content: native scroll preserved (no preventDefault)', prevented === false);
}

// B) at top, pull DOWN past threshold → dismiss
{
  const s = makeSheet({ scrollTop: 0 });
  const prevented = gesture(s.node, s.content, [{ x: 100, y: 100 }, { x: 100, y: 180 }, { x: 100, y: 240 }]);
  check('at top: pull down past threshold dismisses', s.isClosed());
  check('at top: drag owns gesture (preventDefault fired)', prevented === true);
}

// C) at top, small pull DOWN below threshold → spring back, no dismiss
{
  const s = makeSheet({ scrollTop: 0 });
  gesture(s.node, s.content, [{ x: 100, y: 100 }, { x: 100, y: 130 }, { x: 100, y: 150 }]);
  check('at top: small pull below threshold does NOT dismiss', !s.isClosed());
}

// D) from the drag handle, pull DOWN → dismiss even if content not at top
{
  const s = makeSheet({ scrollTop: 500 });
  gesture(s.node, s.handle, [{ x: 100, y: 100 }, { x: 100, y: 200 }, { x: 100, y: 260 }]);
  check('from handle: dismisses regardless of scroll position', s.isClosed());
}

// E) swipe UP → never dismiss
{
  const s = makeSheet({ scrollTop: 0 });
  const prevented = gesture(s.node, s.content, [{ x: 100, y: 300 }, { x: 100, y: 200 }, { x: 100, y: 100 }]);
  check('swipe up does NOT dismiss', !s.isClosed());
  check('swipe up does NOT hijack scroll', prevented === false);
}

// F) horizontal swipe → never dismiss (vertical-only)
{
  const s = makeSheet({ scrollTop: 0 });
  gesture(s.node, s.content, [{ x: 100, y: 100 }, { x: 200, y: 110 }, { x: 300, y: 120 }]);
  check('horizontal swipe does NOT dismiss', !s.isClosed());
}

// G) keyboard retract: active input is blurred when a dismiss drag starts
{
  const s = makeSheet({ scrollTop: 0 });
  let blurred = false;
  const input = new El({ _tag: 'input' });
  input.blur = () => { blurred = true; };
  globalThis.document.activeElement = input;
  gesture(s.node, s.content, [{ x: 100, y: 100 }, { x: 100, y: 180 }, { x: 100, y: 240 }]);
  globalThis.document.activeElement = null;
  check('drag start blurs focused input (keyboard retracts)', blurred);
}

console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
