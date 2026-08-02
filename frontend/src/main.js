import './style.css';
import {
  GetConfig, GetState, SetPower, SetPilot, SetLastState,
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
    <div class="dial-zone" id="dial-zone">
      <div class="dial" id="dial"></div>
      <div class="dial-center">
        <span class="dial-val"><span id="dial-num">–</span><small>%</small></span>
        <span class="dial-lab">brightness</span>
      </div>
    </div>
    <div class="controls">
      <button class="pill on" id="power-pill">⏻ On</button>
    </div>
    <div class="statusline" id="statusline">connecting…</div>
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
  el('dial').style.background = c
    ? `conic-gradient(from ${SWEEP_START}deg, ${c} 0 ${sweep}deg, rgba(255,255,255,.07) ${sweep}deg ${SWEEP_MAX}deg, transparent ${SWEEP_MAX}deg)`
    : `conic-gradient(from ${SWEEP_START}deg, rgba(255,255,255,.07) 0 ${SWEEP_MAX}deg, transparent ${SWEEP_MAX}deg)`;
  el('dial-num').textContent = S.brightness;
  el('dial-zone').classList.toggle('offline', !S.power);

  const alpha = S.power ? 0.06 + (S.brightness / 100) * 0.09 : 0;
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
  el('statusline').innerHTML = parts.join(' &nbsp;·&nbsp; ') || '…';
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
    S.health = res.failed.length
      ? `${res.ok} ok · failed: ${res.failed.join(', ')}`
      : `${res.ok}/${res.ok} ok · ${res.ms}ms`;
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
  let deg = (Math.atan2(dy, dx) * 180) / Math.PI + 90; // 0 = top
  deg = (deg - (SWEEP_START - 180) + 360) % 360;       // rotate into sweep space
  if (deg > SWEEP_MAX) return null;                    // in the dead zone
  return Math.max(1, Math.min(100, Math.round((deg / SWEEP_MAX) * 100)));
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

function setBrightness(v) {
  S.brightness = v;
  S.power = true;
  render();
  debouncedPilot({ dimming: v });
}

// ── power ──────────────────────────────────────────────────────────
el('power-pill').onclick = async () => {
  const next = !S.power;
  S.power = next;
  render();
  const res = await SetPower(S.target.targets, next);
  S.health = res.failed.length
    ? `${res.ok} ok · failed: ${res.failed.join(', ')}`
    : `power ${next ? 'on' : 'off'} · ${res.ms}ms`;
  render();
};

// ── boot ───────────────────────────────────────────────────────────
(async () => {
  S.cfg = await GetConfig();
  S.brightness = S.cfg.lastBrightness > 0 ? S.cfg.lastBrightness : 72;
  S.temp = S.cfg.lastColorTemp > 0 ? S.cfg.lastColorTemp : 4000;
  buildSwitcher();
  render();
})();
