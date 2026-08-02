import './style.css';
import {
  GetConfig, GetState, SetPower, SetPilot, SetLastState,
  Discover, SaveDevice, SetTheme, SaveGroup, DeleteGroup,
} from '../wailsjs/go/main/App';

// ── state ──────────────────────────────────────────────────────────
const S = {
  cfg: { savedDevices: [], groups: [], ip: '', port: '38899' },
  target: null,        // {kind:'group'|'device'|'ip', name, targets:[{ip,port,name}]}
  power: true,
  brightness: 72,
  colorHex: '',
  temp: 4000,
  memberStates: {},    // name -> StateResult
  health: '',
};

// ── dom ────────────────────────────────────────────────────────────
document.querySelector('#app').innerHTML = `
  <span class="blob" id="blob-a"></span>
  <span class="blob" id="blob-b"></span>
  <div class="stage">
    <nav class="switcher" id="switcher"></nav>
    <p class="target-name" id="target-name">—</p>
    <p class="target-sub" id="target-sub"></p>
    <div class="center">
      <div class="cview" id="view-dial">
        <div class="dial-zone" id="dial-zone">
          <svg class="dial-svg" viewBox="0 0 230 230" aria-hidden="true">
            <circle class="dial-track" cx="115" cy="115" r="98" pathLength="360"/>
            <circle id="dial-arc" cx="115" cy="115" r="98" pathLength="360"/>
          </svg>
          <div class="dial-center">
            <span class="dial-val"><span id="dial-num">–</span><small>%</small></span>
            <span class="dial-lab">brightness</span>
          </div>
        </div>
      </div>
      <div class="cview" id="view-temp" hidden>
        <p class="big-val"><span id="temp-num">4000</span><small>K</small></p>
        <input type="range" id="temp-range" min="2200" max="6500" step="100" aria-label="Color temperature" />
        <div class="panel-hint"><span>warm 2200K</span><span>6500K cool</span></div>
      </div>
      <div class="cview" id="view-color" hidden>
        <div class="wheel-zone" id="wheel-zone"><svg id="wheel-svg" viewBox="0 0 190 190" aria-hidden="true"></svg><span class="wheel-dot" id="wheel-dot" hidden></span></div>
        <div class="hexrow" id="hexrow"></div>
      </div>
      <div class="cview" id="view-scenes" hidden><div class="scene-grid" id="scene-grid"></div></div>
      <div class="cview" id="view-timer" hidden>
        <p class="big-val" id="timer-display">–</p>
        <div class="hexrow" id="timer-presets"></div>
        <div class="hexrow" style="margin-top:12px">
          <input type="number" class="name-input" id="timer-mins" min="1" max="720" placeholder="minutes" style="width:110px" />
          <button class="pill" id="timer-start">Start</button>
          <button class="pill" id="timer-cancel" hidden>Cancel</button>
        </div>
      </div>
    </div>
    <div class="controls">
      <button class="pill back" id="back-pill" hidden>← Back</button>
      <button class="pill on" id="power-pill">⏻ On</button>
      <button class="pill" id="temp-pill" data-panel="temp">4000K</button>
      <button class="pill" id="color-pill" data-panel="color">Color</button>
      <button class="pill" id="scenes-pill" data-panel="scenes">Scenes</button>
      <button class="pill" id="timer-pill" data-panel="timer">Timer</button>
    </div>
    <div class="egg-toast" id="egg-toast" hidden></div>
    <div class="statusline" id="statusline">connecting…</div>
    <div class="approw">
      <button class="pill" id="discover-pill">Discover</button>
      <button class="pill" id="groups-pill">Groups</button>
      <button class="pill" id="themes-pill">Themes</button>
    </div>
  </div>
  <div class="egg-dim" id="egg-dim" hidden aria-hidden="true"></div>
  <div class="overlay" id="overlay" hidden>
    <div class="overlay-card">
      <div class="overlay-head"><span id="overlay-title"></span><button class="pill" id="overlay-close">Esc</button></div>
      <div id="overlay-body"></div>
    </div>
  </div>
`;

const el = (id) => document.getElementById(id);

// ── rendering ──────────────────────────────────────────────────────
const SWEEP_START = 220; // degrees, matches design mockup
const SWEEP_MAX = 280;

function lightColor() {
  if (!S.power) return null;
  if (S.colorHex) return S.colorHex;
  // approximate warm..cool white from kelvin 2200-6500
  const t = Math.min(Math.max((S.temp - 2200) / 4300, 0), 1);
  const warm = [255, 217, 160], cool = [224, 240, 250];
  const mix = warm.map((w, i) => Math.round(w + (cool[i] - w) * t));
  return `rgb(${mix.join(',')})`;
}

function render() {
  const c = lightColor();
  const sweep = (S.brightness / 100) * SWEEP_MAX;
  const arc = el('dial-arc');
  arc.style.stroke = c || 'rgba(255,255,255,.07)';
  arc.style.strokeDasharray = `${c ? sweep : 0} ${360 - (c ? sweep : 0)}`;
  el('dial-num').textContent = S.power ? S.brightness : 0; // off reads 0%
  el('dial-zone').classList.toggle('offline', !S.power);

  const alpha = S.power ? 0.14 + (S.brightness / 100) * 0.2 : 0;
  const glow = c || '#FFD9A0';
  el('blob-a').style.background = `radial-gradient(closest-side, ${hexToRgba(glow, alpha)}, transparent 70%)`;
  el('blob-b').style.background = `radial-gradient(closest-side, ${hexToRgba(glow, alpha * 0.6)}, transparent 70%)`;

  const pp = el('power-pill');
  pp.textContent = S.power ? '⏻ On' : '⏻ Off';
  pp.classList.toggle('on', S.power);

  if (S.target) {
    el('target-name').textContent = S.target.name;
    const n = S.target.targets.length;
    const mode = S.colorHex ? `color ${S.colorHex}` : `white ${S.temp}K`;
    el('target-sub').textContent =
      n > 1 ? `${n} lights · ${mode}` : `${mode}`;
  }

  renderStatusLine();
}

// Status line interpolates device names (user/network-sourced) — escape all
// text and whitelist color values before they touch the DOM.
function esc(s) {
  return String(s).replace(/[&<>"']/g, (ch) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]
  ));
}

function safeColor(c) {
  return /^#[0-9A-Fa-f]{6}$/.test(c) ? c : '#FFD9A0';
}

function renderStatusLine() {
  const parts = Object.entries(S.memberStates).map(([name, st]) =>
    st.err
      ? `${esc(name)} <b class="err">offline</b>`
      : `${esc(name)} <b style="color:${safeColor(st.colorHex)}">${st.power ? esc(st.brightness) + '%' : 'off'}</b>`
  );
  if (S.health) parts.push(`<span class="ok">${esc(S.health)}</span>`);
  const hint = Object.values(S.memberStates).find((st) => st.hint)?.hint;
  if (hint) parts.push(`<span class="err">${esc(hint)}</span>`);
  el('statusline').innerHTML = parts.join(' &nbsp;·&nbsp; ') || '…';
}

// One line for a fan-out result: ok count, failures, and the backend's
// actionable hint (e.g. macOS Local Network permission) when present.
function fanoutHealth(res, okMsg) {
  if (!res.failed.length) return okMsg;
  const base = `${res.ok} ok · failed: ${res.failed.join(', ')}`;
  return res.hint ? `${base} — ${res.hint}` : base;
}

function hexToRgba(color, a) {
  if (color.startsWith('rgb')) return color.replace('rgb', 'rgba').replace(')', `,${a})`);
  const n = parseInt(color.slice(1), 16);
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
}

// ── targets ────────────────────────────────────────────────────────
function deviceTarget(d) {
  return { kind: 'device', name: d.name || d.ip, targets: [{ ip: d.ip, port: d.port || S.cfg.port, name: d.name || d.ip }] };
}

function groupTarget(g) {
  const byMac = {};
  for (const d of S.cfg.savedDevices || []) byMac[(d.mac || '').toLowerCase()] = d;
  const targets = (g.macs || [])
    .map((m) => byMac[(m || '').toLowerCase()])
    .filter((d) => d && d.ip)
    .map((d) => ({ ip: d.ip, port: d.port || S.cfg.port, name: d.name || d.ip }));
  return { kind: 'group', name: g.name, targets };
}

function buildSwitcher() {
  const nav = el('switcher');
  nav.innerHTML = '';
  const add = (label, target) => {
    const b = document.createElement('button');
    b.className = 'pill';
    b.textContent = label;
    b.onclick = () => selectTarget(target, b);
    nav.appendChild(b);
    return b;
  };
  const buttons = [];
  for (const g of S.cfg.groups || []) buttons.push(add(g.name, groupTarget(g)));
  for (const d of S.cfg.savedDevices || []) buttons.push(add(d.name || d.ip, deviceTarget(d)));
  if (!buttons.length && S.cfg.ip) {
    buttons.push(add(S.cfg.ip, { kind: 'ip', name: S.cfg.ip, targets: [{ ip: S.cfg.ip, port: S.cfg.port, name: S.cfg.ip }] }));
  }
  if (buttons.length) buttons[0].click();
  else el('statusline').textContent = 'no devices configured — run the TUI once or lumina discover';
}

function selectTarget(target, btn) {
  document.querySelectorAll('.switcher .pill').forEach((p) => p.classList.remove('on'));
  if (btn) btn.classList.add('on');
  S.target = target;
  S.memberStates = {};
  syncStates();
  render();
}

// ── device io ──────────────────────────────────────────────────────
async function syncStates() {
  if (!S.target) return;
  const t0 = S.target;
  await Promise.all(
    t0.targets.map(async (t) => {
      const st = await GetState(t.ip, t.port);
      if (S.target !== t0) return; // stale response, target changed
      S.memberStates[t.name] = st;
      if (!st.err) {
        S.power = st.power;
        if (st.brightness > 0) S.brightness = st.brightness;
        S.colorHex = st.colorHex || '';
        if (st.temp > 0) S.temp = st.temp;
      }
    })
  );
  render();
}

let sendTimer = null;
function debouncedPilot(params) {
  clearTimeout(sendTimer);
  sendTimer = setTimeout(async () => {
    const res = await SetPilot(S.target.targets, params);
    S.health = fanoutHealth(res, `${res.ok}/${res.ok} ok · ${res.ms}ms`);
    SetLastState(S.colorHex, S.brightness, S.temp);
    render();
  }, 140);
}

// ── dial interaction ───────────────────────────────────────────────
const zone = el('dial-zone');
let dragging = false;

function angleToBrightness(e) {
  const r = zone.getBoundingClientRect();
  const dx = e.clientX - (r.left + r.width / 2);
  const dy = e.clientY - (r.top + r.height / 2);
  // clockwise angle from north, 0..360
  const fromNorth = ((Math.atan2(dy, dx) * 180) / Math.PI + 90 + 360) % 360;
  // rotate so the arc start (220 deg from north) is zero
  const rel = (fromNorth - SWEEP_START + 360) % 360;
  if (rel > SWEEP_MAX) {
    // clicks in the bottom gap snap to the nearest end
    return rel - SWEEP_MAX < 360 - rel ? 100 : 1;
  }
  return Math.max(1, Math.min(100, Math.round((rel / SWEEP_MAX) * 100)));
}

zone.addEventListener('pointerdown', (e) => {
  dragging = true;
  zone.setPointerCapture(e.pointerId);
  const v = angleToBrightness(e);
  if (v !== null) setBrightness(v);
});
zone.addEventListener('pointermove', (e) => {
  if (!dragging) return;
  const v = angleToBrightness(e);
  if (v !== null) setBrightness(v);
});
zone.addEventListener('pointerup', () => { dragging = false; });

// ── easter eggs: the dial knows ────────────────────────────────────
let eggTimer = null;
let lastEgg = 0;

const EGG_HTML = {
  1: '<span class="egg-candle">🕯️</span> barely holding on',
  42: '<span class="egg-42">the answer to everything.</span>',
  50: '<span class="egg-50">✋ perfectly balanced, as all things should be</span>',
  66: '<span class="egg-66">EXECUTE ORDER 66.</span>',
  67: '<span class="egg-hand hand-l">🫷</span><span class="egg-67-text">SIX SEVENNN</span><span class="egg-hand hand-r">🫸</span>',
  69: '<span class="egg-nice">nice.</span>',
  88: '<span class="egg-88">88 MPH — GREAT SCOTT! ⚡</span>',
  100: '<span class="egg-100">🌞</span>&nbsp;MAXIMUM POWER',
};

function maybeEgg(v) {
  clearTimeout(eggTimer);
  if (!(v in EGG_HTML)) { lastEgg = 0; return; }
  eggTimer = setTimeout(() => {
    if (S.brightness !== v || lastEgg === v) return; // moved on, or already fired
    lastEgg = v;
    showEgg(v);
  }, 300); // fires on settle, not on drag-through
}

function showEgg(v) {
  const t = el('egg-toast');
  t.innerHTML = EGG_HTML[v];
  t.hidden = false;
  if (v === 66) {
    // the room obeys: brief blackout behind the toast
    const d = el('egg-dim');
    d.hidden = false;
    d.style.animation = 'none';
    void d.offsetWidth; // restart the animation on repeat firings
    d.style.animation = '';
    setTimeout(() => { d.hidden = true; }, 2500);
  }
  setTimeout(() => { t.hidden = true; }, 2500);
}

function setBrightness(v) {
  S.brightness = v;
  maybeEgg(v);
  S.power = true;
  render();
  debouncedPilot({ dimming: v, state: true });
}

// ── expandable panels: temp / color / scenes ───────────────────────
// name, sceneId, representative gradient (what the scene roughly looks like)
const SCENES = [
  ['Ocean', 1, '#0891B2', '#164E63'], ['Romance', 2, '#F472B6', '#9D174D'],
  ['Sunset', 3, '#FB923C', '#B91C1C'], ['Party', 4, '#E879F9', '#4F46E5'],
  ['Fireplace', 5, '#F97316', '#7C2D12'], ['Cozy', 6, '#FDBA74', '#92400E'],
  ['Forest', 7, '#4ADE80', '#14532D'], ['Pastel', 8, '#F9A8D4', '#93C5FD'],
  ['Wake-up', 9, '#FDE68A', '#F59E0B'], ['Bedtime', 10, '#818CF8', '#1E1B4B'],
  ['Daylight', 12, '#FEF3C7', '#FCD34D'], ['Focus', 15, '#E0F2FE', '#7DD3FC'],
];
const PRESET_HEXES = ['#FFD9A0', '#CBA6F7', '#89B4FA', '#A6E3A1', '#F38BA8', '#FFD700', '#FF8C00', '#00FFFF'];

let openPanel = null;

// Per the design: panels replace the dial on the center stage,
// they never stack on top of it.
function togglePanel(name) {
  openPanel = openPanel === name ? null : name;
  el('view-dial').hidden = openPanel !== null;
  el('back-pill').hidden = openPanel === null;
  for (const p of ['temp', 'color', 'scenes', 'timer']) {
    el(`view-${p}`).hidden = p !== openPanel;
    el(`${p}-pill`).classList.toggle('on', p === openPanel);
  }
}

for (const p of ['temp', 'color', 'scenes', 'timer']) {
  el(`${p}-pill`).addEventListener('click', () => togglePanel(p));
}
el('back-pill').addEventListener('click', () => { if (openPanel) togglePanel(openPanel); });

// temp slider
el('temp-range').addEventListener('input', (e) => {
  S.temp = parseInt(e.target.value, 10);
  S.colorHex = '';
  S.power = true;
  el('temp-pill').textContent = `${S.temp}K`;
  el('temp-num').textContent = S.temp;
  render();
  debouncedPilot({ temp: S.temp, dimming: S.brightness, state: true });
});

// color wheel: angle -> hue at fixed s/l, plus preset hex pills
function hslToHex(h, s, l) {
  const f = (n) => {
    const k = (n + h / 30) % 12;
    const a = s * Math.min(l, 1 - l);
    const c = l - a * Math.max(-1, Math.min(k - 3, Math.min(9 - k, 1)));
    return Math.round(255 * c).toString(16).padStart(2, '0');
  };
  return `#${f(0)}${f(8)}${f(4)}`.toUpperCase();
}

function applyColor(hex) {
  S.colorHex = hex;
  S.power = true;
  render();
  buildHexRow();
  const n = parseInt(hex.slice(1), 16);
  debouncedPilot({ r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255, dimming: S.brightness, state: true });
}

// SVG hue ring — WebKit renders CSS radial masks elliptically (same bug the
// dial had), so the ring is 36 stroked arc segments instead.
function buildWheel() {
  const svg = el('wheel-svg');
  const ns = 'http://www.w3.org/2000/svg';
  for (let i = 0; i < 36; i++) {
    const seg = document.createElementNS(ns, 'circle');
    seg.setAttribute('cx', '95');
    seg.setAttribute('cy', '95');
    seg.setAttribute('r', '80');
    seg.setAttribute('pathLength', '360');
    seg.setAttribute('fill', 'none');
    seg.setAttribute('stroke', `hsl(${i * 10}, 100%, 60%)`);
    seg.setAttribute('stroke-width', '26');
    seg.setAttribute('stroke-dasharray', '10.6 349.4');
    seg.setAttribute('stroke-dashoffset', String(-i * 10));
    svg.appendChild(seg);
  }
}

const wheelZone = el('wheel-zone');
let wheelDragging = false;

function wheelPick(e) {
  const r = wheelZone.getBoundingClientRect();
  const dx = e.clientX - (r.left + r.width / 2);
  const dy = e.clientY - (r.top + r.height / 2);
  const hue = ((Math.atan2(dy, dx) * 180) / Math.PI + 360) % 360;
  const dot = el('wheel-dot');
  const radius = r.width / 2 - 14;
  dot.hidden = false;
  dot.style.left = `${r.width / 2 + Math.cos((hue * Math.PI) / 180) * radius - 7}px`;
  dot.style.top = `${r.height / 2 + Math.sin((hue * Math.PI) / 180) * radius - 7}px`;
  applyColor(hslToHex(hue, 1, 0.6));
}

wheelZone.addEventListener('pointerdown', (e) => { wheelDragging = true; wheelZone.setPointerCapture(e.pointerId); wheelPick(e); });
wheelZone.addEventListener('pointermove', (e) => { if (wheelDragging) wheelPick(e); });
wheelZone.addEventListener('pointerup', () => { wheelDragging = false; });

function buildHexRow() {
  const row = el('hexrow');
  row.innerHTML = '';
  const hexes = [...new Set([S.cfg.lastColor, ...PRESET_HEXES].filter((h) => /^#[0-9A-Fa-f]{6}$/.test(h || '')))];
  for (const h of hexes.slice(0, 8)) {
    const b = document.createElement('button');
    b.className = 'pill hex-pill';
    b.textContent = h.toUpperCase();
    b.style.color = h;
    b.style.borderColor = h + '55';
    b.style.background = '#11111B';
    if (h.toUpperCase() === (S.colorHex || '').toUpperCase()) b.classList.add('on');
    b.onclick = () => applyColor(h.toUpperCase());
    row.appendChild(b);
  }
}

// scenes grid
function buildScenes() {
  const grid = el('scene-grid');
  grid.innerHTML = '';
  for (const [name, id, c1, c2] of SCENES) {
    const b = document.createElement('button');
    b.className = 'pill scene-pill';
    b.textContent = name;
    // each pill previews its scene's look
    b.style.background = `linear-gradient(135deg, ${c1}59, ${c2}E6)`;
    b.style.borderColor = `${c1}59`;
    b.style.color = c1;
    b.onclick = async () => {
      grid.querySelectorAll('.scene-pill').forEach((p) => {
        p.classList.remove('on');
        p.style.boxShadow = '';
      });
      b.classList.add('on');
      b.style.boxShadow = `0 0 14px ${c1}66`;
      S.power = true;
      const res = await SetPilot(S.target.targets, { sceneId: id, state: true });
      S.health = fanoutHealth(res, `scene ${name} · ${res.ms}ms`);
      render();
    };
    grid.appendChild(b);
  }
}

// ── sleep timer ────────────────────────────────────────────────────
// ponytail: in-app timer only — dies if the app quits. The TUI/CLI covers
// detached timers (lumina --timer / cron).
const T = { until: 0, handle: null, targetName: '' };

function timerTick() {
  if (!T.until) { el('timer-display').textContent = '–'; return; }
  const left = Math.max(0, T.until - Date.now());
  const m = Math.floor(left / 60000);
  const s = Math.floor((left % 60000) / 1000);
  el('timer-display').textContent = `${m}:${String(s).padStart(2, '0')}`;
}
setInterval(timerTick, 1000);

function startTimer(mins) {
  cancelTimer();
  const targets = S.target.targets;
  T.until = Date.now() + mins * 60000;
  T.targetName = S.target.name;
  T.handle = setTimeout(async () => {
    const res = await SetPower(targets, false);
    S.power = false;
    S.health = `sleep: ${T.targetName} off (${res.ok} ok)`;
    cancelTimer();
    render();
  }, mins * 60000);
  el('timer-cancel').hidden = false;
  el('timer-pill').textContent = `Timer ·`;
  S.health = `sleep in ${mins}m → ${T.targetName}`;
  timerTick();
  render();
}

function cancelTimer() {
  if (T.handle) clearTimeout(T.handle);
  T.handle = null;
  T.until = 0;
  el('timer-cancel').hidden = true;
  el('timer-pill').textContent = 'Timer';
  timerTick();
}

for (const mins of [15, 30, 60]) {
  const b = document.createElement('button');
  b.className = 'pill';
  b.textContent = `${mins}m`;
  b.onclick = () => startTimer(mins);
  el('timer-presets').appendChild(b);
}
el('timer-start').onclick = () => {
  const mins = parseInt(el('timer-mins').value, 10);
  if (mins >= 1 && mins <= 720) startTimer(mins);
};
el('timer-cancel').onclick = () => { cancelTimer(); S.health = 'sleep timer cancelled'; render(); };

// ── power ──────────────────────────────────────────────────────────
el('power-pill').onclick = async () => {
  const next = !S.power;
  S.power = next;
  render();
  const res = await SetPower(S.target.targets, next);
  S.health = fanoutHealth(res, `power ${next ? 'on' : 'off'} · ${res.ms}ms`);
  render();
};

// ── overlays: discover + themes ────────────────────────────────────
function openOverlay(title) {
  el('overlay-title').textContent = title;
  el('overlay-body').innerHTML = '';
  el('overlay').hidden = false;
  return el('overlay-body');
}
function closeOverlay() { el('overlay').hidden = true; }
el('overlay-close').onclick = closeOverlay;
el('overlay').addEventListener('click', (e) => { if (e.target === el('overlay')) closeOverlay(); });
window.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  if (!el('overlay').hidden) { closeOverlay(); return; }
  if (openPanel) togglePanel(openPanel); // Esc collapses back to the home dial
});

el('discover-pill').onclick = async () => {
  const body = openOverlay('Discover');
  body.innerHTML = '<p class="overlay-hint">scanning 255.255.255.255:38899 …</p>';
  const devices = await Discover();
  body.innerHTML = '';
  if (!devices.length) {
    body.innerHTML = '<p class="overlay-hint">no bulbs found — same network as the bulbs?</p>';
    return;
  }
  // overlay the user's saved names by MAC — never show/overwrite them with
  // the firmware module name
  const savedByMac = {};
  for (const s of S.cfg.savedDevices || []) savedByMac[(s.mac || '').toLowerCase()] = s;
  for (const d of devices) {
    const card = document.createElement('div');
    card.className = 'dev-card';
    const saved = savedByMac[(d.mac || '').toLowerCase()];
    const known = !!saved;
    const displayName = (saved && saved.name) || d.name || d.ip;
    const title = document.createElement('div');
    title.className = 'dev-title';
    title.textContent = displayName;
    const badge = document.createElement('span');
    badge.className = 'dev-badge' + (known ? ' saved' : '');
    badge.textContent = known ? 'SAVED' : 'NEW';
    title.appendChild(badge);
    const meta = document.createElement('div');
    meta.className = 'dev-meta';
    meta.textContent = `${d.ip} · ${d.mac || 'no mac'}`;
    const actions = document.createElement('div');
    actions.className = 'dev-actions';
    const save = document.createElement('button');
    save.className = 'pill';
    save.textContent = known ? 'Rename…' : 'Save…';
    // No prompt()/alert() in the Wails webview — inline editor instead.
    save.onclick = () => {
      actions.innerHTML = '';
      const input = document.createElement('input');
      input.className = 'name-input';
      input.placeholder = 'Device name';
      input.value = displayName;
      input.maxLength = 32;
      const ok = document.createElement('button');
      ok.className = 'pill';
      ok.textContent = 'Save';
      const cancel = document.createElement('button');
      cancel.className = 'pill';
      cancel.textContent = 'Cancel';
      const commit = async () => {
        const name = input.value.trim();
        if (!name) { input.focus(); return; }
        try {
          // Go errors surface as promise rejections through Wails bindings
          await SaveDevice({ name, ip: d.ip, port: S.cfg.port, mac: d.mac });
        } catch (err) {
          meta.textContent = String(err);
          return;
        }
        S.cfg = await GetConfig();
        buildSwitcher();
        closeOverlay();
      };
      ok.onclick = commit;
      input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') commit();
        if (e.key === 'Escape') { e.stopPropagation(); cancel.onclick(); }
      });
      cancel.onclick = () => {
        actions.innerHTML = '';
        actions.appendChild(save);
      };
      actions.append(input, ok, cancel);
      input.focus();
      input.select();
    };
    actions.appendChild(save);
    card.append(title, meta, actions);
    body.appendChild(card);
  }
};

// ── groups management ──────────────────────────────────────────────
function renderGroupsOverlay() {
  const body = openOverlay('Groups');

  // create row
  const createRow = document.createElement('div');
  createRow.className = 'dev-actions';
  createRow.style.marginBottom = '14px';
  const nameInput = document.createElement('input');
  nameInput.className = 'name-input';
  nameInput.placeholder = 'New group name';
  nameInput.maxLength = 32;
  const createBtn = document.createElement('button');
  createBtn.className = 'pill';
  createBtn.textContent = 'Create';
  createBtn.onclick = async () => {
    const name = nameInput.value.trim();
    if (!name) { nameInput.focus(); return; }
    if ((S.cfg.groups || []).some((g) => g.name === name)) { nameInput.select(); return; }
    try { await SaveGroup({ name, macs: [] }); } catch (e) { S.health = String(e); render(); return; }
    S.cfg = await GetConfig();
    buildSwitcher();
    renderGroupsOverlay(); // re-render with the new group open for editing
  };
  nameInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') createBtn.onclick(); });
  createRow.append(nameInput, createBtn);
  body.appendChild(createRow);

  if (!(S.cfg.groups || []).length) {
    const hint = document.createElement('p');
    hint.className = 'overlay-hint';
    hint.textContent = 'no groups yet — create one, then tick its members';
    body.appendChild(hint);
    return;
  }

  for (const g of S.cfg.groups) {
    const card = document.createElement('div');
    card.className = 'dev-card';

    const title = document.createElement('div');
    title.className = 'dev-title';
    title.textContent = g.name;
    const count = document.createElement('span');
    count.className = 'dev-badge saved';
    count.textContent = `${(g.macs || []).length} member(s)`;
    title.appendChild(count);
    const del = document.createElement('button');
    del.className = 'pill';
    del.textContent = 'Delete';
    del.style.marginLeft = 'auto';
    del.onclick = async () => {
      await DeleteGroup(g.name);
      S.cfg = await GetConfig();
      buildSwitcher();
      renderGroupsOverlay();
    };
    title.appendChild(del);
    card.appendChild(title);

    // member checkboxes from saved devices
    const inGroup = new Set((g.macs || []).map((m) => (m || '').toLowerCase()));
    for (const d of S.cfg.savedDevices || []) {
      const mac = (d.mac || '').toLowerCase();
      const row = document.createElement('label');
      row.className = 'member-row';
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.checked = inGroup.has(mac);
      cb.onchange = async () => {
        const macs = new Set(inGroup);
        if (cb.checked) macs.add(mac); else macs.delete(mac);
        try { await SaveGroup({ name: g.name, macs: [...macs] }); } catch (e) { S.health = String(e); render(); return; }
        S.cfg = await GetConfig();
        buildSwitcher();
        renderGroupsOverlay();
      };
      const label = document.createElement('span');
      label.textContent = d.name || d.ip;
      row.append(cb, label);
      card.appendChild(row);
    }
    if (!(S.cfg.savedDevices || []).length) {
      const hint = document.createElement('div');
      hint.className = 'dev-meta';
      hint.textContent = 'no saved devices — discover and save bulbs first';
      card.appendChild(hint);
    }
    body.appendChild(card);
  }
}

el('groups-pill').onclick = renderGroupsOverlay;

// One toggle: dark or light. Persisted as mocha/latte so the TUI's theme
// system understands the same config value.
function applyMode(light) {
  document.body.classList.toggle('light', light);
  el('themes-pill').textContent = light ? '☾ Night' : '⛅ Dusk';
}

el('themes-pill').onclick = () => {
  const light = !document.body.classList.contains('light');
  applyMode(light);
  S.cfg.theme = light ? 'latte' : 'mocha';
  SetTheme(S.cfg.theme);
};

// ── boot ───────────────────────────────────────────────────────────
(async () => {
  S.cfg = await GetConfig();
  S.brightness = S.cfg.lastBrightness > 0 ? S.cfg.lastBrightness : 72;
  S.temp = S.cfg.lastColorTemp > 0 ? S.cfg.lastColorTemp : 4000;
  el('temp-range').value = S.temp;
  el('temp-pill').textContent = `${S.temp}K`;
  applyMode(S.cfg.theme === 'latte');
  buildSwitcher();
  buildWheel();
  buildHexRow();
  buildScenes();
  render();

  // 10s heartbeat keeps the window honest when the bulb changes from
  // elsewhere (phone app, TUI, cron). Paused while the window is hidden;
  // skipped mid-drag so a stale read can't fight the user's hand.
  setInterval(() => {
    if (document.hidden || dragging || wheelDragging) return;
    syncStates();
  }, 10_000);
})();
