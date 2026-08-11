import './style.css';
import {
  GetConfig, GetState, SetPower, SetPilot, SetLastState,
  Discover, SaveDevice, DeleteDevice, SetTheme, SaveGroup, DeleteGroup,
  CheckUpdate, OpenReleases,
} from '../wailsjs/go/main/App';

// ── state ──────────────────────────────────────────────────────────
const S = {
  cfg: { savedDevices: [], groups: [], ip: '', port: '38899' },
  updateTag: '',      // non-empty release tag = newer version available
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
  <span class="paintlayer" id="paint-a"></span>
  <span class="paintlayer" id="paint-b"></span>
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
        <div class="temp-marks" aria-hidden="true"><span style="left:11.6%">warm</span><span style="left:41.9%">day</span><span style="left:100%">cool</span></div>
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
  arc.style.stroke = S.dawnEgg && c && !S.colorHex ? 'url(#dawn)' : (c || 'rgba(255,255,255,.07)');
  arc.style.strokeDasharray = `${c ? sweep : 0} ${360 - (c ? sweep : 0)}`;
  const knob = el('dial-knob');
  if (knob) {
    const a = (sweep * Math.PI) / 180;
    knob.setAttribute('cx', String(115 + 98 * Math.cos(a)));
    knob.setAttribute('cy', String(115 + 98 * Math.sin(a)));
    knob.style.display = c ? '' : 'none';
  }
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

  // scene playing = whole-app state: the Scenes pill morphs into the
  // indicator (scene color, live dot, ✕ to stop) wherever you are
  const sp = el('scenes-pill');
  if (S.sceneId) {
    sp.classList.add('sceneplay');
    sp.style.borderColor = S.sceneColor + '8C';
    sp.style.color = S.sceneColor;
    sp.innerHTML = `<span class="live" style="background:${S.sceneColor}"></span>${esc(S.sceneName)}<span class="x">✕</span>`;
  } else if (sp.classList.contains('sceneplay')) {
    sp.classList.remove('sceneplay');
    sp.style.borderColor = '';
    sp.style.color = '';
    sp.textContent = 'Scenes';
  }

  // external changes (phone app, TUI) must reach every control — the temp
  // pill and slider are only otherwise written by their own input handler
  el('temp-pill').textContent = `${S.temp}K`;
  const tr = el('temp-range');
  if (document.activeElement !== tr) tr.value = S.temp;
  el('temp-num').textContent = S.temp;

  renderStatusLine();
}

// Status line interpolates device names (user/network-sourced) — escape all
// text and whitelist color values before they touch the DOM.
function esc(s) {
  return String(s).replace(/[&<>"']/g, (ch) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]
  ));
}

function renderStatusLine() {
  const parts = Object.entries(S.memberStates).map(([name, st]) =>
    st.err
      ? `<span class="chip"><i class="dot dot-offline"></i>${esc(name)} · offline</span>`
      : `<span class="chip"><i class="dot ${st.power ? 'dot-on' : 'dot-off'}"></i>${esc(name)} · ${st.power ? esc(st.brightness) + '%' : 'off'}</span>`
  );
  if (S.health) parts.push(`<span class="ok">${esc(S.health)}</span>`);
  if (S.updateTag) parts.push(`<button class="chip update" id="update-chip" title="Open release page"><span class="up">⬆</span> ${esc(S.updateTag)} available</button>`);
  const hint = Object.values(S.memberStates).find((st) => st.hint)?.hint;
  if (hint) parts.push(`<span class="err">${esc(hint)}</span>`);
  const states = Object.values(S.memberStates);
  if (states.length && states.every((st) => !st.err && !st.power)) {
    parts.push('<span class="void">the void stares back 🌑</span>');
  }
  el('statusline').innerHTML = parts.join(' ') || '…';
  const uc = document.getElementById('update-chip');
  if (uc) uc.onclick = () => OpenReleases();
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
  return { kind: 'device', name: d.name || d.ip, targets: [{ ip: d.ip, port: d.port || S.cfg.port, name: d.name || d.ip, mac: d.mac || '' }] };
}

function groupTarget(g) {
  const byMac = {};
  for (const d of S.cfg.savedDevices || []) byMac[(d.mac || '').toLowerCase()] = d;
  const targets = (g.macs || [])
    .map((m) => byMac[(m || '').toLowerCase()])
    .filter((d) => d && d.ip)
    .map((d) => ({ ip: d.ip, port: d.port || S.cfg.port, name: d.name || d.ip, mac: d.mac || '' }));
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

// Commands don't wait for the 10s heartbeat: every successful send writes
// the new state straight into memberStates so the chips update instantly.
function reflectLocal(failed = []) {
  for (const t of (S.target && S.target.targets) || []) {
    if (failed.includes(t.name)) continue;
    const st = S.memberStates[t.name] || (S.memberStates[t.name] = {});
    Object.assign(st, { err: '', power: S.power, brightness: S.brightness, colorHex: S.colorHex, temp: S.temp });
  }
}

// Backend heals stale saved-device IPs by MAC mid-send; when it reports a
// heal, refresh our in-memory target IPs so the next command skips the
// fail-then-rediscover cycle. In-place update — no switcher rebuild, no
// selection jump.
async function refreshTargetIPs() {
  S.cfg = await GetConfig();
  const byMac = {};
  for (const d of S.cfg.savedDevices || []) byMac[(d.mac || '').toLowerCase()] = d;
  for (const t of (S.target && S.target.targets) || []) {
    const d = byMac[(t.mac || '').toLowerCase()];
    if (d && d.ip) t.ip = d.ip;
  }
}

async function sendPilot(targets, params) {
  const res = await SetPilot(targets, params);
  if (res.healed) refreshTargetIPs();
  return res;
}

async function sendPower(targets, on) {
  const res = await SetPower(targets, on);
  if (res.healed) refreshTargetIPs();
  return res;
}

let sendTimer = null;
let pilotSeq = 0;
function debouncedPilot(params) {
  clearTimeout(sendTimer);
  sendTimer = setTimeout(async () => {
    const seq = ++pilotSeq;
    const t0 = S.target;
    const res = await sendPilot(t0.targets, params);
    // a heal can hold a send for seconds; drop responses the user outran
    if (seq !== pilotSeq || S.target !== t0) return;
    S.health = fanoutHealth(res, `${res.ok}/${res.ok} ok · ${res.ms}ms`);
    SetLastState(S.colorHex, S.brightness, S.temp);
    reflectLocal(res.failed);
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

// clicks near a tick land exactly on it; drags stay precise
function snapTick(v) {
  for (const m of [25, 50, 75]) if (Math.abs(v - m) <= 2) return m;
  return v;
}

zone.addEventListener('pointerdown', (e) => {
  dragging = true;
  zone.setPointerCapture(e.pointerId);
  const v = angleToBrightness(e);
  if (v !== null) setBrightness(snapTick(v));
});
zone.addEventListener('pointermove', (e) => {
  if (!dragging) return;
  const v = angleToBrightness(e);
  if (v !== null) setBrightness(v);
});
zone.addEventListener('pointerup', () => { dragging = false; });

// mouse wheel over the dial nudges brightness like a real knob
zone.addEventListener('wheel', (e) => {
  e.preventDefault();
  setBrightness(Math.max(1, Math.min(100, S.brightness - Math.sign(e.deltaY) * 2)));
}, { passive: false });

// keyboard: arrows nudge brightness (shift = 10), space toggles power.
// Ignored while typing or while an overlay is up.
window.addEventListener('keydown', (e) => {
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName) || !el('overlay').hidden) return;
  const step = e.shiftKey ? 10 : 1;
  if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
    e.preventDefault();
    setBrightness(Math.max(1, Math.min(100, S.brightness + (e.key === 'ArrowRight' ? step : -step))));
  } else if (e.key === ' ') {
    e.preventDefault();
    el('power-pill').click();
  }
});

// ── easter eggs: the dial knows ────────────────────────────────────
let eggTimer = null;
let lastEgg = 0;

const EGG_HTML = {
  1: '<span class="egg-candle">🕯️</span> barely holding on',
  11: '<span class="egg-100">🎸</span>&nbsp;these go to eleven',
  13: '<span class="egg-candle">🕯️</span> unlucky thirteen',
  21: '🃏 blackjack',
  33: '<span class="egg-33">💿</span> 33⅓ RPM — drop the needle',
  42: '<span class="egg-42">the answer to everything.</span>',
  50: '<span class="egg-50">✋ perfectly balanced, as all things should be</span>',
  66: '<span class="egg-66">EXECUTE ORDER 66.</span>',
  67: '<span class="egg-hand hand-l">🫷</span><span class="egg-67-text">SIX SEVENNN</span><span class="egg-hand hand-r">🫸</span>',
  69: '<span class="egg-nice">nice.</span>',
  77: '<span class="egg-77">🎰 jackpot</span>',
  88: '<span class="egg-88">88 MPH — GREAT SCOTT! ⚡</span>',
  99: '99 little bugs in the code 🐛',
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

// generic egg toast — also used by the non-dial eggs below
function toastEgg(html, ms = 2500) {
  const t = el('egg-toast');
  t.innerHTML = html;
  t.hidden = false;
  clearTimeout(toastEgg._h);
  toastEgg._h = setTimeout(() => { t.hidden = true; }, ms);
}

function showEgg(v) {
  toastEgg(EGG_HTML[v]);
  if (v === 66 || v === 13) {
    // the room obeys: brief blackout behind the toast
    const d = el('egg-dim');
    d.hidden = false;
    d.style.animation = 'none';
    void d.offsetWidth; // restart the animation on repeat firings
    d.style.animation = '';
    setTimeout(() => { d.hidden = true; }, 2500);
  }
}

function confetti() {
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  const colors = ['#E879F9', '#4F46E5', '#FFD9A0', '#A6E3A1', '#F38BA8', '#89B4FA'];
  for (let i = 0; i < 40; i++) {
    const p = document.createElement('span');
    p.className = 'confetti';
    p.style.left = `${Math.random() * 100}vw`;
    p.style.background = colors[i % colors.length];
    p.style.animationDelay = `${Math.random() * 0.6}s`;
    p.style.animationDuration = `${1.6 + Math.random()}s`;
    document.body.appendChild(p);
    setTimeout(() => p.remove(), 3400);
  }
}

// konami code fires the Party scene on the current target
const KONAMI = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];
let konamiIdx = 0;
window.addEventListener('keydown', (e) => {
  konamiIdx = e.key === KONAMI[konamiIdx] ? konamiIdx + 1 : (e.key === KONAMI[0] ? 1 : 0);
  if (konamiIdx !== KONAMI.length) return;
  konamiIdx = 0;
  if (!S.target) return;
  S.power = true;
  sendPilot(S.target.targets, { sceneId: 4, state: true });
  toastEgg('🎮 KONAMI — party mode');
  confetti();
  render();
});

function setBrightness(v) {
  S.brightness = v;
  maybeEgg(v);
  maybeSupernova(v);
  S.power = true;
  render();
  debouncedPilot({ dimming: v, state: true });
}

// hold the dial at 100 for 3s: one-shot ray burst around the ring
let novaTimer = null;
function maybeSupernova(v) {
  clearTimeout(novaTimer);
  if (v !== 100) return;
  novaTimer = setTimeout(() => {
    if (S.brightness !== 100 || !S.power) return;
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    zone.classList.add('supernova');
    setTimeout(() => zone.classList.remove('supernova'), 2600);
  }, 3000);
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
  if (S.temp === 6500) toastEgg('❄️ arctic mode');
  if (S.temp === 2200) fireplaceEmbers();
  render();
  debouncedPilot({ temp: S.temp, dimming: S.brightness, state: true });
});

// 2200K floor: embers drift up from the warm end of the slider
function fireplaceEmbers() {
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  if (!fireplaceEmbers._toasted) {
    fireplaceEmbers._toasted = true;
    toastEgg('🔥 fireplace mode');
  }
  const view = el('view-temp');
  for (let i = 0; i < 14; i++) {
    const e = document.createElement('i');
    e.className = 'ember';
    e.style.left = `${Math.random() * 16}%`;
    e.style.width = e.style.height = `${1.5 + Math.random() * 3}px`;
    e.style.background = `rgba(255,${140 + (Math.random() * 60 | 0)},60,${0.4 + Math.random() * 0.4})`;
    e.style.animationDelay = `${Math.random() * 0.8}s`;
    view.appendChild(e);
    setTimeout(() => e.remove(), 3200);
  }
}

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

// 25/50/75 markers: dots punched into the track centerline. A light dot
// under the arc shows on the unfilled track; a dark twin on top reads as a
// dimple where the arc covers it. No numbers — the center readout has the
// exact value. A knob at the arc tip (positioned by render) says "drag me".
function buildDialTicks() {
  const svg = document.querySelector('.dial-svg');
  const ns = 'http://www.w3.org/2000/svg';
  const arc = el('dial-arc');
  for (const f of [0.25, 0.5, 0.75]) {
    const a = (f * SWEEP_MAX * Math.PI) / 180; // svg-local, clockwise from east
    const x = String(115 + 98 * Math.cos(a));
    const y = String(115 + 98 * Math.sin(a));
    for (const [cls, before] of [['dial-dot-under', true], ['dial-dot-over', false]]) {
      const c = document.createElementNS(ns, 'circle');
      c.setAttribute('cx', x);
      c.setAttribute('cy', y);
      c.setAttribute('r', '2.4');
      c.setAttribute('class', cls);
      if (before) svg.insertBefore(c, arc); else svg.appendChild(c);
    }
  }
  const knob = document.createElementNS(ns, 'circle');
  knob.setAttribute('id', 'dial-knob');
  knob.setAttribute('r', '5.5');
  svg.appendChild(knob);
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
  buildScenes._party = buildScenes._party || 0;
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
    // ponytail: highlight tracks the last scene WE set — device can't report
    // its active scene (wiz lib state has no sceneId; upstream change to lift)
    if (S.sceneId === id) {
      b.classList.add('on');
      b.style.boxShadow = `0 0 14px ${c1}66`;
    }
    b.onclick = async () => {
      grid.querySelectorAll('.scene-pill').forEach((p) => {
        p.classList.remove('on');
        p.style.boxShadow = '';
      });
      b.classList.add('on');
      b.style.boxShadow = `0 0 14px ${c1}66`;
      S.sceneId = id;
      S.sceneName = name;
      S.sceneColor = c1;
      if (name === 'Party' && ++buildScenes._party % 3 === 0) confetti();
      S.power = true;
      const res = await sendPilot(S.target.targets, { sceneId: id, state: true });
      S.health = fanoutHealth(res, `scene ${name} · ${res.ms}ms`);
      reflectLocal(res.failed);
      render();
    };
    grid.appendChild(b);
  }
}

// a running scene has no off switch on the bulb — stopping it means
// sending plain white, which overrides the scene program
async function stopScene() {
  S.sceneId = null;
  S.sceneName = '';
  S.sceneColor = '';
  document.querySelectorAll('.scene-pill').forEach((p) => { p.classList.remove('on'); p.style.boxShadow = ''; });
  S.colorHex = '';
  S.power = true;
  const res = await sendPilot(S.target.targets, { temp: S.temp, dimming: S.brightness, state: true });
  S.health = fanoutHealth(res, `scene stopped · back to white ${S.temp}K`);
  reflectLocal(res.failed);
  render();
}

// capture phase so the ✕ stops the scene without also toggling the panel
el('scenes-pill').addEventListener('click', (e) => {
  if (e.target.classList.contains('x')) {
    e.stopPropagation();
    stopScene();
  }
}, true);

// ── sleep timer ────────────────────────────────────────────────────
// ponytail: in-app timer only — dies if the app quits. The TUI/CLI covers
// detached timers (lumina --timer / cron).
const T = { until: 0, handle: null, targetName: '' };

function timerTick() {
  if (!T.until) { el('timer-display').textContent = '–'; return; }
  const left = Math.max(0, T.until - Date.now());
  const m = Math.floor(left / 60000);
  const s = Math.floor((left % 60000) / 1000);
  const end = new Date(T.until);
  const hh = String(end.getHours()).padStart(2, '0');
  const mm = String(end.getMinutes()).padStart(2, '0');
  el('timer-display').textContent = `${m}:${String(s).padStart(2, '0')} → ${hh}:${mm}`;
  if (m === 0 && s === 7 && !T.egg007) {
    T.egg007 = true;
    toastEgg('🍸 007 — shaken, not stirred');
  }
}
setInterval(timerTick, 1000);

function startTimer(mins) {
  cancelTimer();
  const targets = S.target.targets;
  T.egg007 = false;
  T.until = Date.now() + mins * 60000;
  T.targetName = S.target.name;
  T.handle = setTimeout(async () => {
    const res = await sendPower(targets, false);
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
let powerClicks = [];
el('power-pill').onclick = async () => {
  const now = Date.now();
  powerClicks = powerClicks.filter((t) => now - t < 3000).concat(now);
  if (powerClicks.length >= 5) {
    powerClicks = [];
    toastEgg('🤨 make up your mind');
  }
  const next = !S.power;
  S.power = next;
  render();
  const res = await sendPower(S.target.targets, next);
  S.health = fanoutHealth(res, `power ${next ? 'on' : 'off'} · ${res.ms}ms`);
  reflectLocal(res.failed);
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

let lastScan = []; // kept so delete/rename can re-render without a rescan

function renderDiscoverCards(body) {
  body.innerHTML = '';
  // overlay the user's saved names by MAC — never show/overwrite them with
  // the firmware module name
  const savedByMac = {};
  for (const s of S.cfg.savedDevices || []) savedByMac[(s.mac || '').toLowerCase()] = s;
  const foundMacs = new Set(lastScan.map((d) => (d.mac || '').toLowerCase()));
  // saved devices the scan didn't find still get a card, so they can be
  // renamed or deleted while offline
  const offlineSaved = (S.cfg.savedDevices || [])
    .filter((s) => !foundMacs.has((s.mac || '').toLowerCase()))
    .map((s) => ({ ip: s.ip, mac: s.mac, name: s.name, offline: true }));
  const all = [...lastScan, ...offlineSaved];
  if (!all.length) {
    body.innerHTML = '<p class="overlay-hint">no bulbs found — same network as the bulbs?</p>';
    return;
  }
  for (const d of all) {
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
    meta.textContent = `${d.ip} · ${d.mac || 'no mac'}${d.offline ? ' · offline' : ''}`;
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
        renderDiscoverCards(body);
      };
      actions.append(input, ok, cancel);
      input.focus();
      input.select();
    };
    actions.appendChild(save);
    if (known) {
      // two-click confirm — the webview has no confirm() dialog
      const del = document.createElement('button');
      del.className = 'pill';
      del.textContent = 'Delete';
      del.onclick = async () => {
        if (del.textContent !== 'Sure?') { del.textContent = 'Sure?'; return; }
        await DeleteDevice(d.mac);
        S.cfg = await GetConfig();
        buildSwitcher();
        renderDiscoverCards(body);
      };
      actions.appendChild(del);
    }
    card.append(title, meta, actions);
    body.appendChild(card);
  }
}

el('discover-pill').onclick = async () => {
  const body = openOverlay('Discover');
  body.innerHTML = '<p class="overlay-hint">scanning 255.255.255.255:38899 …</p>';
  lastScan = await Discover();
  syncStates();
  renderDiscoverCards(body);
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
    // two-tap: first tap arms for 3s, second tap actually deletes
    del.onclick = async () => {
      if (!del.classList.contains('confirm')) {
        del.classList.add('confirm');
        del.textContent = 'sure? tap again';
        setTimeout(() => { del.classList.remove('confirm'); del.textContent = 'Delete'; }, 3000);
        return;
      }
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

// ── themes: eight moods ────────────────────────────────────────────
// Stored under names the TUI's theme system understands where one exists
// (night→mocha, dusk→latte, catppuccin/dracula/gruvbox as-is); indigo and
// ember are desktop-only, the TUI falls back to its default for those.
const THEMES = {
  night:     { store: 'mocha',     bg: ['#0A0A0F', '#020203'], accent: '#FFD9A0', text: '#F8F8F4', label: '☾ Night' },
  dusk:      { store: 'latte',     bg: ['#575072', '#35304A'], accent: '#FFC6A0', text: '#F5F2FA', label: '⛅ Dusk', light: true },
  macchiato: { store: 'macchiato', bg: ['#24273A', '#181926'], accent: '#C6A0F6', text: '#CAD3F5', label: 'Macchiato' },
  frappe:    { store: 'frappe',    bg: ['#303446', '#232634'], accent: '#CA9EE6', text: '#C6D0F5', label: 'Frappé' },
  dracula:   { store: 'dracula',   bg: ['#282A36', '#1D1E26'], accent: '#BD93F9', text: '#F8F8F2', label: 'Dracula' },
  gruvbox:   { store: 'gruvbox',   bg: ['#282828', '#1D2021'], accent: '#D3869B', text: '#EBDBB2', label: 'Gruvbox' },
  indigo:    { store: 'indigo',    bg: ['#0F172A', '#020617'], accent: '#818CF8', text: '#E2E8F0', label: 'Indigo' },
  ember:     { store: 'ember',     bg: ['#1C120C', '#0A0503'], accent: '#FF9E64', text: '#F5E9DF', label: 'Ember' },
};

function themeKeyFromStore(name) {
  if (name === 'mocha') return 'night';
  if (name === 'latte') return 'dusk';
  return THEMES[name] ? name : 'night';
}

function applyThemeKey(key) {
  const t = THEMES[key];
  const r = document.documentElement.style;
  r.setProperty('--accent', t.accent);
  r.setProperty('--text', t.text);
  document.body.style.background = `linear-gradient(180deg, ${t.bg[0]}, ${t.bg[1]})`;
  document.body.classList.toggle('light', !!t.light);
  el('themes-pill').textContent = t.label;
  S.themeKey = key;
}

el('themes-pill').onclick = () => {
  const body = openOverlay('Themes');
  const grid = document.createElement('div');
  grid.className = 'scene-grid';
  for (const [key, t] of Object.entries(THEMES)) {
    const b = document.createElement('button');
    b.className = 'pill theme-pill';
    b.textContent = t.label;
    b.style.color = t.accent;
    b.style.borderColor = t.accent + '55';
    b.style.background = `linear-gradient(135deg, ${t.bg[0]}, ${t.bg[1]})`;
    if (key === S.themeKey) b.classList.add('on');
    b.onclick = () => {
      applyThemeKey(key);
      S.cfg.theme = t.store;
      SetTheme(t.store);
      grid.querySelectorAll('.theme-pill').forEach((p) => p.classList.remove('on'));
      b.classList.add('on');
    };
    grid.appendChild(b);
  }
  body.appendChild(grid);
};

// ── time-of-day eggs: the app knows what hour it is ────────────────
// midnight (00:xx): stars in the background. dawn (05-06:xx): the arc
// takes a sunrise gradient until the next launch.
function timeEggs() {
  const h = new Date().getHours();
  if (h === 0) {
    const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
    for (let i = 0; i < 40; i++) {
      const s = document.createElement('i');
      s.className = 'star';
      s.style.left = `${Math.random() * 100}%`;
      s.style.top = `${Math.random() * 60}%`;
      s.style.width = s.style.height = `${1 + Math.random() * 2}px`;
      s.style.opacity = `${0.25 + Math.random() * 0.5}`;
      if (!still) s.style.animationDelay = `${Math.random() * 4}s`;
      document.body.appendChild(s);
    }
    toastEgg('🌌 past midnight — the stars are out', 3500);
  } else if (h >= 5 && h < 7) {
    const svg = document.querySelector('.dial-svg');
    const ns = 'http://www.w3.org/2000/svg';
    const defs = document.createElementNS(ns, 'defs');
    const grad = document.createElementNS(ns, 'linearGradient');
    grad.setAttribute('id', 'dawn');
    grad.setAttribute('x1', '0'); grad.setAttribute('y1', '0');
    grad.setAttribute('x2', '1'); grad.setAttribute('y2', '1');
    for (const [off, col] of [['0', '#ffb88a'], ['0.55', '#f6c8d8'], ['1', '#bcc8f5']]) {
      const stop = document.createElementNS(ns, 'stop');
      stop.setAttribute('offset', off);
      stop.setAttribute('stop-color', col);
      grad.appendChild(stop);
    }
    defs.appendChild(grad);
    svg.prepend(defs);
    S.dawnEgg = true;
    toastEgg('🌅 early bird — dawn palette until sunrise', 3500);
  }
}

// ── idle lightpainting: after 45s untouched the UI recedes and two light
// pools in the bulbs' current color take the room; any input wakes it ──
let idleTimer = null;

function idleEnter() {
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  const c = lightColor();
  if (!c) return; // all off — nothing to paint with
  el('paint-a').style.background = `radial-gradient(closest-side, ${hexToRgba(c, 0.5)}, transparent 70%)`;
  el('paint-b').style.background = `radial-gradient(closest-side, ${hexToRgba(c, 0.42)}, transparent 70%)`;
  document.body.classList.add('paint');
}

function idleReset() {
  // waking from idle re-checks the bulbs — they may have changed while painting
  if (document.body.classList.contains('paint')) syncStates();
  document.body.classList.remove('paint');
  clearTimeout(idleTimer);
  idleTimer = setTimeout(idleEnter, 45_000);
}

for (const ev of ['pointermove', 'pointerdown', 'keydown', 'wheel']) {
  window.addEventListener(ev, idleReset, { passive: true });
}
idleReset();

// ── boot ───────────────────────────────────────────────────────────
(async () => {
  S.cfg = await GetConfig();
  S.brightness = S.cfg.lastBrightness > 0 ? S.cfg.lastBrightness : 72;
  S.temp = S.cfg.lastColorTemp > 0 ? S.cfg.lastColorTemp : 4000;
  el('temp-range').value = S.temp;
  el('temp-pill').textContent = `${S.temp}K`;
  applyThemeKey(themeKeyFromStore(S.cfg.theme));
  buildSwitcher();
  buildDialTicks();
  timeEggs();
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
  // window coming back to front syncs immediately instead of waiting a tick
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) syncStates();
  });

  // update chip: check on launch, then daily while running; "" = up to date
  const checkUpdate = async () => {
    S.updateTag = await CheckUpdate();
    renderStatusLine();
  };
  checkUpdate();
  setInterval(checkUpdate, 24 * 60 * 60 * 1000);
})();
