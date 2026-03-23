package web

var baseHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Jolie's Diary</title>
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🔒</text></svg>">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Playfair+Display:ital,wght@0,400;0,700;1,400;1,700&family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
<style>
:root {
  --bg: #0d0810;
  --sidebar: #110d1a;
  --card: #1c1028;
  --border: #3a1d48;
  --pink: #e8779a;
  --pink-dim: #c45a7a;
  --purple: #c084fc;
  --text: #f0e6ff;
  --text-muted: #a78db0;
  --success: #86efac;
  --danger: #f87171;
  --warning: #fbbf24;
  --sw: 240px;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: 'Inter', sans-serif;
  font-size: 14px;
  min-height: 100vh;
  display: flex;
}
a { color: var(--pink); text-decoration: none; }
a:hover { color: var(--purple); }

/* ── Sidebar ── */
.sidebar {
  width: var(--sw);
  background: var(--sidebar);
  border-right: 1px solid var(--border);
  position: fixed;
  top: 0; left: 0; bottom: 0;
  display: flex;
  flex-direction: column;
  padding: 24px 0;
  z-index: 10;
}
.brand {
  padding: 0 20px 24px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 16px;
}
.brand-name {
  font-family: 'Playfair Display', serif;
  font-style: italic;
  font-size: 20px;
  color: var(--pink);
  display: block;
}
.brand-sub { font-size: 11px; color: var(--text-muted); margin-top: 4px; display: block; }
.nav {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 0 12px;
}
.nav-link {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  transition: all 0.15s;
  border-left: 2px solid transparent;
}
.nav-link:hover { background: rgba(232,119,154,0.08); color: var(--pink); }
.nav-link.active {
  background: rgba(232,119,154,0.13);
  color: var(--pink);
  border-left-color: var(--pink);
}
.nav-icon { width: 20px; text-align: center; font-size: 15px; }
.sidebar-foot {
  padding: 16px 12px 0;
  border-top: 1px solid var(--border);
  margin: 16px 0 0;
}
.tg-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(36,129,204,0.12);
  border: 1px solid rgba(36,129,204,0.28);
  color: #74b9e6;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.15s;
}
.tg-btn:hover { background: rgba(36,129,204,0.22); color: #74b9e6; }

/* ── Main ── */
.main {
  margin-left: var(--sw);
  flex: 1;
  padding: 36px 44px;
}
.page-hd { margin-bottom: 28px; }
.page-title {
  font-family: 'Playfair Display', serif;
  font-size: 26px;
  font-weight: 700;
}
.page-sub { color: var(--text-muted); font-size: 13px; margin-top: 4px; }

/* ── Cards ── */
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px 24px;
}
.card-title {
  font-family: 'Playfair Display', serif;
  font-style: italic;
  font-size: 13px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 16px;
}

/* ── Stat cards ── */
.stats-grid { display: grid; gap: 14px; margin-bottom: 22px; }
.g5 { grid-template-columns: repeat(5, 1fr); }
.g4 { grid-template-columns: repeat(4, 1fr); }
.g3 { grid-template-columns: repeat(3, 1fr); }
.g2 { grid-template-columns: repeat(2, 1fr); }
.stat-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 18px 20px;
}
.stat-lbl {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
  margin-bottom: 8px;
}
.stat-val {
  font-family: 'Playfair Display', serif;
  font-size: 34px;
  font-weight: 700;
  line-height: 1;
}
.stat-sub { font-size: 11px; color: var(--text-muted); margin-top: 5px; }
.c-pink { color: var(--pink); }
.c-purple { color: var(--purple); }
.c-green { color: var(--success); }
.c-red { color: var(--danger); }
.c-yellow { color: var(--warning); }
.c-muted { color: var(--text-muted); }

/* ── Badges ── */
.badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.badge-pink   { background: rgba(232,119,154,0.13); color: var(--pink);    border: 1px solid rgba(232,119,154,0.28); }
.badge-purple { background: rgba(192,132,252,0.12); color: var(--purple);  border: 1px solid rgba(192,132,252,0.28); }
.badge-success{ background: rgba(134,239,172,0.10); color: var(--success); border: 1px solid rgba(134,239,172,0.28); }
.badge-danger { background: rgba(248,113,113,0.10); color: var(--danger);  border: 1px solid rgba(248,113,113,0.28); }
.badge-warning{ background: rgba(251,191,36,0.10);  color: var(--warning); border: 1px solid rgba(251,191,36,0.28); }
.badge-muted  { background: rgba(167,141,176,0.10); color: var(--text-muted); border: 1px solid rgba(167,141,176,0.18); }

/* ── Grid layouts ── */
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }

/* ── Table ── */
.data-table { width: 100%; border-collapse: collapse; }
.data-table th {
  text-align: left;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--text-muted);
  padding: 0 12px 12px;
  border-bottom: 1px solid var(--border);
}
.data-table td {
  padding: 11px 12px;
  border-bottom: 1px solid rgba(58,29,72,0.45);
  color: var(--text);
  font-size: 13px;
  vertical-align: top;
}
.data-table tr:last-child td { border-bottom: none; }
.data-table tr:hover td { background: rgba(232,119,154,0.025); }
.desc-cell { max-width: 360px; line-height: 1.5; }
.no-wrap { white-space: nowrap; }

/* ── Progress bar ── */
.prog-bar { height: 5px; background: var(--border); border-radius: 3px; overflow: hidden; margin: 6px 0; }
.prog-fill { height: 100%; border-radius: 3px; background: var(--pink); }
.prog-green { background: var(--success); }

/* ── Timeline ── */
.timeline { display: flex; flex-direction: column; gap: 10px; }
.tl-item {
  display: flex;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--card);
}
.tl-granted { border-color: rgba(134,239,172,0.2); background: rgba(134,239,172,0.025); }
.tl-denied  { border-color: rgba(248,113,113,0.2); background: rgba(248,113,113,0.025); }
.tl-icon { font-size: 22px; flex-shrink: 0; padding-top: 1px; }
.tl-body { flex: 1; min-width: 0; }
.tl-hd { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; flex-wrap: wrap; }
.tl-title { font-weight: 600; font-size: 13px; }
.tl-date { font-size: 11px; color: var(--text-muted); margin-left: auto; }
.tl-msg { font-size: 12px; color: var(--text-muted); line-height: 1.5; margin-bottom: 4px; font-style: italic; }
.tl-resp { font-size: 12px; color: var(--text); line-height: 1.5; }
.tl-meta { font-size: 11px; color: var(--text-muted); margin-top: 6px; display: flex; gap: 12px; }

/* ── Toy grid ── */
.toy-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 14px; }
.toy-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  transition: border-color 0.15s;
}
.toy-card:hover { border-color: var(--pink); }
.toy-card.in-use { border-color: var(--pink); box-shadow: 0 0 14px rgba(232,119,154,0.18); }
.toy-img { width: 100%; aspect-ratio: 1; object-fit: cover; display: block; }
.toy-placeholder {
  width: 100%; aspect-ratio: 1;
  display: flex; align-items: center; justify-content: center;
  font-size: 52px;
  background: #140b1f;
}
.toy-info { padding: 12px; }
.toy-name { font-weight: 600; color: var(--text); margin-bottom: 4px; }
.toy-desc { font-size: 11px; color: var(--text-muted); line-height: 1.4; }
.toy-foot {
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  display: flex; align-items: center; justify-content: space-between;
}

/* ── Calendar ── */
.cal-nav { display: flex; align-items: center; gap: 14px; margin-bottom: 22px; }
.cal-month-title {
  font-family: 'Playfair Display', serif;
  font-size: 22px;
  font-style: italic;
  flex: 1;
}
.cal-btn {
  background: var(--card);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 7px 16px;
  border-radius: 8px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
  font-family: 'Inter', sans-serif;
}
.cal-btn:hover { border-color: var(--pink); color: var(--pink); }
.cal-legend {
  display: flex; gap: 14px; margin-bottom: 16px; flex-wrap: wrap;
  font-size: 12px; color: var(--text-muted); align-items: center;
}
.legend-swatch {
  width: 12px; height: 12px; border-radius: 3px;
  display: inline-block; margin-right: 4px; vertical-align: middle;
}
.cal-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 5px; }
.cal-day-hd {
  text-align: center;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--text-muted);
  padding: 6px 0 10px;
}
.cal-cell {
  min-height: 76px;
  border-radius: 8px;
  padding: 8px;
  font-size: 13px;
  display: flex;
  flex-direction: column;
}
.cal-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 4px; }
.cal-num { font-weight: 700; font-size: 15px; line-height: 1; }
.cal-hours { font-size: 10px; color: var(--text-muted); font-weight: 500; margin-top: 1px; }
.cal-body { flex: 1; }
.cal-foot { display: flex; gap: 4px; align-items: center; flex-wrap: wrap; margin-top: auto; padding-top: 4px; }
.ci { font-size: 12px; line-height: 1; display: inline-flex; align-items: center; gap: 2px; }
.ci-task { font-size: 11px; font-weight: 700; }
.ci-done { color: var(--success); }
.ci-fail { color: var(--danger); }
.ci-pend { color: var(--warning); }
.ci-og { color: #f9c74f; } /* gold */
.ci-od { color: var(--text-muted); }
.ci-count { font-size: 10px; font-weight: 600; }

.cal-empty { background: transparent; }
.cal-free {
  background: rgba(167,141,176,0.04);
  border: 1px solid rgba(58,29,72,0.28);
}
.cal-free .cal-num { color: var(--text-muted); }
.cal-locked {
  background: rgba(232,119,154,0.07);
  border: 1px solid rgba(232,119,154,0.22);
}
.cal-locked .cal-num { color: var(--pink); }
.cal-locked .cal-hours { color: rgba(232,119,154,0.6); }
.cal-done {
  background: rgba(134,239,172,0.08);
  border: 1px solid rgba(134,239,172,0.28);
}
.cal-done .cal-num { color: var(--success); }
.cal-done .cal-hours { color: rgba(134,239,172,0.6); }
.cal-failed {
  background: rgba(248,113,113,0.08);
  border: 1px solid rgba(248,113,113,0.28);
}
.cal-failed .cal-num { color: var(--danger); }
.cal-failed .cal-hours { color: rgba(248,113,113,0.6); }
.cal-today { box-shadow: 0 0 0 2px var(--warning) !important; }
.cal-today .cal-num { color: var(--warning) !important; }

/* ── Countdown ── */
.cd-unit { display:flex; flex-direction:column; align-items:center; }
.cd-num {
  font-family: 'Playfair Display', serif;
  font-size: 38px; font-weight: 700;
  color: var(--text-muted);
  line-height: 1; min-width: 2ch; text-align: center;
}
.cd-lbl { font-size: 9px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.07em; margin-top: 3px; }
.cd-sep { font-size: 28px; color: var(--border); padding-bottom: 14px; font-weight: 300; }

/* ── Lock hero ── */
.lock-hero {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 28px 36px;
  display: flex;
  align-items: center;
  gap: 28px;
  margin-bottom: 22px;
  position: relative;
  overflow: hidden;
}
.lock-hero::after {
  content: '';
  position: absolute;
  top: -60px; right: -60px;
  width: 220px; height: 220px;
  background: radial-gradient(circle, rgba(232,119,154,0.1) 0%, transparent 70%);
  pointer-events: none;
}
.lock-emoji { font-size: 60px; line-height: 1; }
.lock-info { flex: 1; }
.lock-days {
  font-family: 'Playfair Display', serif;
  font-size: 52px;
  font-weight: 700;
  color: var(--pink);
  line-height: 1;
}
.lock-lbl { color: var(--text-muted); font-size: 14px; margin-top: 4px; }
.lock-badges { display: flex; gap: 7px; margin-top: 14px; flex-wrap: wrap; }

/* ── Current task ── */
.cur-task {
  background: linear-gradient(135deg, #1c1028 0%, #1a0f22 100%);
  border: 1px solid rgba(232,119,154,0.28);
  border-radius: 12px;
  padding: 18px 22px;
  margin-bottom: 22px;
}
.cur-task-hd {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 10px;
}
.cur-task-lbl {
  font-family: 'Playfair Display', serif;
  font-size: 12px;
  font-weight: 700;
  color: var(--pink);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.cur-task-desc { color: var(--text); line-height: 1.6; }
.cur-task-meta { margin-top: 10px; font-size: 12px; color: var(--text-muted); }

/* ── Empty state ── */
.empty {
  text-align: center; padding: 44px 20px;
  color: var(--text-muted);
}
.empty-icon { font-size: 36px; margin-bottom: 10px; }
.empty-text { font-size: 13px; }
.empty-sub { font-size: 12px; color: var(--text-muted); margin-top: 6px; }

/* ── Filter buttons (shared across pages) ── */
.filter-btn {
  background: var(--card);
  border: 1px solid var(--border);
  color: var(--text-muted);
  padding: 5px 14px;
  border-radius: 20px;
  cursor: pointer;
  font-size: 12px;
  font-family: 'Inter', sans-serif;
  transition: all .15s;
}
.filter-btn:hover { border-color: var(--pink); color: var(--pink); }
.filter-btn.active { background: rgba(232,119,154,.15); border-color: var(--pink); color: var(--pink); }

/* ── Responsive ── */
@media (max-width: 768px) {
  :root { --sw: 0px; }
  .sidebar {
    transform: translateX(-100%);
    transition: transform 0.25s;
    width: 240px;
    z-index: 100;
  }
  .sidebar.open { transform: translateX(0); }
  .main { margin-left: 0; padding: 16px 18px; }
  .g5 { grid-template-columns: repeat(2, 1fr); }
  .g4 { grid-template-columns: repeat(2, 1fr); }
  .g3 { grid-template-columns: repeat(2, 1fr); }
  .grid-2 { grid-template-columns: 1fr; }
  .lock-hero { flex-direction: column; gap: 16px; }
  .lock-hero > div:last-child { border-left: none !important; padding-left: 0 !important; margin-left: 0 !important; border-top: 1px solid var(--border); padding-top: 14px !important; }
  .menu-btn { display: flex; }
  .cal-grid { font-size: 11px; }
  .cal-cell { min-height: 52px; padding: 5px; }
  .cal-num { font-size: 12px; }
  .gal-grid { columns: 2 120px; }
}
@media (min-width: 769px) {
  .menu-btn { display: none; }
}
.menu-btn {
  position: fixed; top: 14px; left: 14px; z-index: 200;
  background: var(--card); border: 1px solid var(--border);
  color: var(--text); width: 38px; height: 38px;
  border-radius: 8px; cursor: pointer; font-size: 18px;
  align-items: center; justify-content: center;
}
.sidebar-backdrop {
  display: none; position: fixed; inset: 0; background: rgba(0,0,0,.5); z-index: 50;
}
.sidebar-backdrop.open { display: block; }
</style>
</head>
<body>

<button class="menu-btn" id="menu-btn" aria-label="Menú">☰</button>
<div class="sidebar-backdrop" id="sidebar-backdrop"></div>

<aside class="sidebar" id="sidebar">
  <div class="brand">
    <span class="brand-name">Jolie's Diary</span>
    <span class="brand-sub">castidad &amp; obediencia</span>
  </div>
  <nav class="nav">
    <a href="/" class="nav-link {{if eq .Nav "dashboard"}}active{{end}}">
      <span class="nav-icon">🔒</span> Estado
    </a>
    <a href="/calendar" class="nav-link {{if eq .Nav "calendar"}}active{{end}}">
      <span class="nav-icon">📅</span> Calendario
    </a>
    <a href="/tasks" class="nav-link {{if eq .Nav "tasks"}}active{{end}}">
      <span class="nav-icon">📋</span> Órdenes
    </a>
    <a href="/chatasks" class="nav-link {{if eq .Nav "chatasks"}}active{{end}}">
      <span class="nav-icon">🌐</span> Comunidad
    </a>
    <a href="/permissions" class="nav-link {{if eq .Nav "permissions"}}active{{end}}">
      <span class="nav-icon">🌸</span> Permisos
    </a>
    <a href="/toys" class="nav-link {{if eq .Nav "toys"}}active{{end}}">
      <span class="nav-icon">🎀</span> Inventario
    </a>
    <a href="/wardrobe" class="nav-link {{if eq .Nav "wardrobe"}}active{{end}}">
      <span class="nav-icon">👗</span> Guardarropa
    </a>
    <a href="/gallery" class="nav-link {{if eq .Nav "gallery"}}active{{end}}">
      <span class="nav-icon">🖼️</span> Galería
    </a>
    <a href="/contract" class="nav-link {{if eq .Nav "contract"}}active{{end}}">
      <span class="nav-icon">📜</span> Contrato
    </a>
    <a href="/checkins" class="nav-link {{if eq .Nav "checkins"}}active{{end}}">
      <span class="nav-icon">📸</span> Check-ins
    </a>
  </nav>
  <div class="sidebar-foot">
    <a href="{{.TelegramLink}}" target="_blank" class="tg-btn">
      ✈️ Abrir Telegram
    </a>
  </div>
</aside>

<main class="main">
  {{template "content" .}}
</main>

<script>
(function(){
  var btn = document.getElementById('menu-btn');
  var sidebar = document.getElementById('sidebar');
  var backdrop = document.getElementById('sidebar-backdrop');
  if (!btn) return;
  function openSidebar() { sidebar.classList.add('open'); backdrop.classList.add('open'); }
  function closeSidebar() { sidebar.classList.remove('open'); backdrop.classList.remove('open'); }
  btn.addEventListener('click', openSidebar);
  backdrop.addEventListener('click', closeSidebar);
})();
</script>
</body>
</html>`

// ── Dashboard ──────────────────────────────────────────────────────────────

var dashboardHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Estado actual</h1>
  <p class="page-sub">Tu progreso en castidad y obediencia</p>
</div>

{{if .IsLocked}}
<div class="lock-hero">
  <div class="lock-emoji">🔒</div>
  <div class="lock-info" style="flex:1;display:flex;gap:0;align-items:stretch;min-width:0;">

    <!-- ── Columna izquierda: contadores ── -->
    <div style="flex:1;min-width:0;">

      <!-- Contador ascendente -->
      <div style="margin-bottom:4px;">
        <div style="font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.09em;color:var(--pink);margin-bottom:8px;display:flex;align-items:center;gap:5px;">
          <span>▲</span> tiempo encerrada
        </div>
        {{if .LockStartISO}}
        <div id="cu-wrap" data-start="{{.LockStartISO}}" style="display:flex;align-items:flex-end;gap:5px;">
          <div class="cd-unit"><span class="cd-num c-pink" id="cu-d">—</span><span class="cd-lbl">días</span></div>
          <span class="cd-sep" style="color:rgba(232,119,154,0.45);">:</span>
          <div class="cd-unit"><span class="cd-num c-pink" id="cu-h">——</span><span class="cd-lbl">horas</span></div>
          <span class="cd-sep" style="color:rgba(232,119,154,0.45);">:</span>
          <div class="cd-unit"><span class="cd-num c-pink" id="cu-m">——</span><span class="cd-lbl">min</span></div>
          <span class="cd-sep" style="color:rgba(232,119,154,0.45);">:</span>
          <div class="cd-unit"><span class="cd-num c-pink" id="cu-s">——</span><span class="cd-lbl">seg</span></div>
        </div>
        {{else}}
        <div style="display:flex;align-items:baseline;gap:6px;">
          <span class="lock-days">{{.DaysLocked}}</span>
          <span style="color:var(--text-muted);font-size:13px;">días</span>
        </div>
        {{end}}
      </div>

      <!-- Contador descendente -->
      {{if .HasEndDate}}
      <div style="padding-top:14px;border-top:1px solid rgba(58,29,72,0.6);margin-top:10px;">
        <div style="font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.09em;color:var(--text-muted);margin-bottom:8px;display:flex;align-items:center;gap:5px;">
          <span>▼</span> tiempo restante
        </div>
        <div id="cd-wrap" data-end="{{.LockEndISO}}" data-start="{{.LockStartISO}}" style="display:flex;align-items:flex-end;gap:5px;">
          <div class="cd-unit"><span class="cd-num" id="cd-d">—</span><span class="cd-lbl">días</span></div>
          <span class="cd-sep">:</span>
          <div class="cd-unit"><span class="cd-num" id="cd-h">——</span><span class="cd-lbl">horas</span></div>
          <span class="cd-sep">:</span>
          <div class="cd-unit"><span class="cd-num" id="cd-m">——</span><span class="cd-lbl">min</span></div>
          <span class="cd-sep">:</span>
          <div class="cd-unit"><span class="cd-num" id="cd-s">——</span><span class="cd-lbl">seg</span></div>
        </div>
        <div style="margin-top:12px;">
          <div style="display:flex;justify-content:space-between;font-size:11px;color:var(--text-muted);margin-bottom:5px;">
            <span>{{formatDatePtr .LockStartDate}}</span>
            <span>{{.ProgressPct}}% completado</span>
            <span>{{formatDatePtr .LockEndDate}}</span>
          </div>
          <div class="prog-bar" style="height:5px;">
            <div class="prog-fill" id="lock-prog" style="{{pctStyle .ProgressPct 100}}"></div>
          </div>
        </div>
      </div>
      {{end}}

      <!-- Badges -->
      <div class="lock-badges" style="margin-top:16px;">
        <span class="badge badge-muted">intensidad {{.Intensity}}</span>
        {{if .WeeklyDebt}}<span class="badge badge-danger">⚠ Deuda: {{.WeeklyDebt}}h</span>{{end}}
        {{if .PendingCheckin}}<span class="badge badge-warning">📸 Check-in pendiente</span>{{end}}
      </div>
    </div>

    <!-- ── Columna derecha: rendimiento ── -->
    <div style="border-left:1px solid var(--border);padding-left:24px;margin-left:24px;min-width:168px;display:flex;flex-direction:column;gap:16px;justify-content:center;">

      <!-- Obediencia -->
      <div>
        <div style="font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.08em;color:var(--text-muted);margin-bottom:6px;">Obediencia</div>
        <div style="font-size:13px;font-weight:600;color:var(--purple);line-height:1.3;margin-bottom:7px;">{{.ObedienceName}}</div>
        <div style="display:flex;gap:5px;align-items:center;">
          <span style="font-size:13px;{{if ge .ObedienceLevel 1}}color:var(--purple){{else}}color:var(--border){{end}};">●</span>
          <span style="font-size:13px;{{if ge .ObedienceLevel 2}}color:var(--purple){{else}}color:var(--border){{end}};">●</span>
          <span style="font-size:13px;{{if ge .ObedienceLevel 3}}color:var(--purple){{else}}color:var(--border){{end}};">●</span>
          <span style="font-size:13px;{{if ge .ObedienceLevel 4}}color:var(--purple){{else}}color:var(--border){{end}};">●</span>
          <span style="font-size:11px;color:var(--text-muted);margin-left:4px;">🔥 {{.Streak}} racha</span>
        </div>
      </div>

      <!-- Tareas -->
      <div style="border-top:1px solid rgba(58,29,72,0.5);padding-top:14px;">
        <div style="font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.08em;color:var(--text-muted);margin-bottom:8px;">Tareas</div>
        <div style="display:flex;gap:14px;margin-bottom:7px;">
          <span style="font-size:20px;font-weight:700;color:var(--success);font-family:'Playfair Display',serif;">{{.TasksCompleted}}<span style="font-size:11px;margin-left:2px;font-family:'Inter',sans-serif;">✓</span></span>
          <span style="font-size:20px;font-weight:700;color:var(--danger);font-family:'Playfair Display',serif;">{{.TasksFailed}}<span style="font-size:11px;margin-left:2px;font-family:'Inter',sans-serif;">✗</span></span>
        </div>
        <div class="prog-bar" style="height:4px;">
          <div class="prog-fill prog-green" style="{{pctStyle .TasksCompleted (add .TasksCompleted .TasksFailed)}}"></div>
        </div>
        <div style="font-size:10px;color:var(--text-muted);margin-top:4px;">{{.CompletionRate}}% completadas</div>
      </div>

      <!-- Permisos -->
      <div style="border-top:1px solid rgba(58,29,72,0.5);padding-top:14px;">
        <div style="font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:.08em;color:var(--text-muted);margin-bottom:6px;">Permisos</div>
        {{if lt .DaysSinceOrgasm 0}}
        <div style="font-size:20px;font-weight:700;color:var(--purple);font-family:'Playfair Display',serif;">∞</div>
        <div style="font-size:10px;color:var(--text-muted);margin-top:2px;">nunca concedido</div>
        {{else}}
        <div style="font-size:20px;font-weight:700;color:var(--purple);font-family:'Playfair Display',serif;">{{.DaysSinceOrgasm}}<span style="font-size:12px;font-family:'Inter',sans-serif;">d</span></div>
        <div style="font-size:10px;color:var(--text-muted);margin-top:2px;">sin orgasmo · {{.GrantRate}}% aprobación</div>
        {{end}}
      </div>

    </div>
  </div>
</div>

{{if .HasActiveEvent}}
<div style="background:rgba(192,132,252,0.08);border:1px solid rgba(192,132,252,0.28);border-radius:10px;padding:12px 18px;display:flex;align-items:center;gap:12px;margin-bottom:22px;">
  <span style="font-size:22px;">{{if eq .ActiveEventType "freeze"}}🧊{{else if eq .ActiveEventType "hidetime"}}🕶️{{else if eq .ActiveEventType "pillory"}}🏚️{{else}}⚡{{end}}</span>
  <div>
    <span style="font-weight:600;color:var(--purple);">Evento activo: {{if eq .ActiveEventType "freeze"}}Congelada{{else if eq .ActiveEventType "hidetime"}}Tiempo oculto{{else if eq .ActiveEventType "pillory"}}Picota{{else}}{{.ActiveEventType}}{{end}}</span>
    <span style="font-size:12px;color:var(--text-muted);margin-left:10px;">expira en {{.ActiveEventExpires}}</span>
  </div>
</div>
{{end}}

{{else}}
<div class="lock-hero">
  <div class="lock-emoji">🔓</div>
  <div class="lock-info">
    <div class="lock-days c-muted">Libre</div>
    <div class="lock-lbl">sin cerradura activa</div>
  </div>
</div>
{{end}}

<div class="stats-grid g4">
  <div class="stat-card">
    <div class="stat-lbl">Tareas completadas</div>
    <div class="stat-val c-green">{{.TasksCompleted}}</div>
    <div class="stat-sub">{{.CompletionRate}}% tasa de éxito</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Tareas fallidas</div>
    <div class="stat-val c-red">{{.TasksFailed}}</div>
    <div class="stat-sub">esta sesión</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Racha actual</div>
    <div class="stat-val c-pink">{{.Streak}}</div>
    <div class="stat-sub">obediencia {{.ObedienceName}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Deuda semanal</div>
    <div class="stat-val {{if .WeeklyDebt}}c-red{{else}}c-green{{end}}">{{.WeeklyDebt}}h</div>
    <div class="stat-sub">{{if .WeeklyDebt}}pendiente de pagar{{else}}al día ✓{{end}}</div>
  </div>
</div>

<div class="stats-grid g5" style="margin-top:-8px;">
  <div class="stat-card">
    <div class="stat-lbl">Permisos concedidos</div>
    <div class="stat-val c-green">{{.OrgasmGranted}}</div>
    <div class="stat-sub">{{.GrantRate}}% aprobación</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Sesiones juguetes</div>
    <div class="stat-val c-yellow">{{.OrgasmToys}}</div>
    <div class="stat-sub">de {{.OrgasmTotal}} solicitados</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Permisos negados</div>
    <div class="stat-val c-red">{{.OrgasmDenied}}</div>
    <div class="stat-sub">sin orgasmo</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Sin eyacular</div>
    {{if lt .DaysSinceOrgasm 0}}
    <div class="stat-val" style="color:var(--purple);">∞</div>
    <div class="stat-sub">nunca concedido</div>
    {{else}}
    <div class="stat-val" style="color:var(--purple);">{{.DaysSinceOrgasm}}d</div>
    <div class="stat-sub">desde el último orgasmo</div>
    {{end}}
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Tiempo neto</div>
    {{if gt .TimeAdded .TimeRemoved}}
    <div class="stat-val c-red">+{{.TimeAdded}}h</div>
    <div class="stat-sub">añadido / −{{.TimeRemoved}}h quitado</div>
    {{else if gt .TimeRemoved 0}}
    <div class="stat-val c-green">−{{.TimeRemoved}}h</div>
    <div class="stat-sub">+{{.TimeAdded}}h añadido</div>
    {{else}}
    <div class="stat-val c-muted">0h</div>
    <div class="stat-sub">sin cambios de tiempo</div>
    {{end}}
  </div>
</div>

{{if .HasCurrentTask}}
<div class="cur-task">
  <div class="cur-task-hd">
    <span class="cur-task-lbl">Tarea activa</span>
    <span class="badge badge-warning">pendiente</span>
  </div>
  <div class="cur-task-desc">{{.CurrentTaskDesc}}</div>
  <div class="cur-task-meta">Vence: {{formatDate .CurrentTaskDue}}</div>
</div>
{{end}}

{{if .HasTodayOutfit}}
<div class="card" style="border-color:var(--pink); margin-bottom:22px;">
  <div style="display:flex; gap:16px; align-items:flex-start; flex-wrap:wrap;">
    {{if and .OutfitConfirmed .TodayOutfitPhotoURL}}
    <img src="{{safeURL .TodayOutfitPhotoURL}}" alt="Outfit"
      style="width:80px; height:110px; object-fit:cover; border-radius:8px; border:1px solid var(--border); flex-shrink:0;">
    {{else}}
    <div style="width:80px; height:110px; border-radius:8px; background:var(--sidebar); border:1px solid var(--border); display:flex; align-items:center; justify-content:center; font-size:30px; flex-shrink:0;">👗</div>
    {{end}}
    <div style="flex:1; min-width:160px;">
      <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
        <span style="font-size:14px; font-weight:600; color:var(--pink);">Outfit de hoy</span>
        {{if .OutfitConfirmed}}
        <span class="badge badge-success">✅ confirmado</span>
        {{else}}
        <span class="badge badge-warning">⏳ pendiente</span>
        {{end}}
      </div>
      <p style="color:var(--text); font-size:13px; line-height:1.5; margin-bottom:6px;">{{.TodayOutfitDesc}}</p>
      {{if .TodayPoseDesc}}<p style="font-size:12px; color:var(--text-muted);"><span style="color:var(--pink);">🧍</span> {{.TodayPoseDesc}}</p>{{end}}
      {{if and .OutfitConfirmed .TodayOutfitComment}}
      <p style="font-size:12px; color:var(--purple); font-style:italic; margin-top:6px; line-height:1.5;">&#8220;{{.TodayOutfitComment}}&#8221;</p>
      {{end}}
    </div>
  </div>
</div>
{{end}}

<div class="grid-2">
  <div class="card">
    <div class="card-title">Últimas órdenes</div>
    {{if .RecentTasks}}
    <table class="data-table">
      <thead>
        <tr>
          <th>Fecha</th>
          <th>Descripción</th>
          <th>Estado</th>
        </tr>
      </thead>
      <tbody>
        {{range .RecentTasks}}
        <tr>
          <td class="no-wrap c-muted">{{formatShort .AssignedAt}}</td>
          <td class="desc-cell">{{truncate .Description 55}}</td>
          <td>{{statusBadge .Status}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <div class="empty">
      <div class="empty-icon">📋</div>
      <div class="empty-text">Sin tareas todavía</div>
    </div>
    {{end}}
  </div>

  <div class="card">
    <div class="card-title">Permisos de orgasmo</div>
    {{if .OrgasmTotal}}
    <div style="margin-bottom:16px;">
      <div style="display:flex;justify-content:space-between;font-size:12px;color:var(--text-muted);margin-bottom:4px;">
        <span>Tasa de concesión</span>
        <span>{{.GrantRate}}%</span>
      </div>
      <div class="prog-bar">
        <div class="prog-fill prog-green" style="{{pctStyle .OrgasmGranted .OrgasmTotal}}"></div>
      </div>
    </div>
    <div style="display:flex;gap:10px;margin-bottom:14px;">
      <div style="flex:1;text-align:center;padding:12px 8px;background:rgba(134,239,172,0.05);border-radius:8px;border:1px solid rgba(134,239,172,0.14);">
        <div style="font-size:26px;font-weight:700;color:var(--success);font-family:'Playfair Display',serif;">{{.OrgasmGranted}}</div>
        <div style="font-size:11px;color:var(--text-muted);margin-top:2px;">concedidos</div>
      </div>
      {{if .OrgasmToys}}
      <div style="flex:1;text-align:center;padding:12px 8px;background:rgba(251,191,36,0.05);border-radius:8px;border:1px solid rgba(251,191,36,0.14);">
        <div style="font-size:26px;font-weight:700;color:var(--warning);font-family:'Playfair Display',serif;">{{.OrgasmToys}}</div>
        <div style="font-size:11px;color:var(--text-muted);margin-top:2px;">sesiones</div>
      </div>
      {{end}}
      <div style="flex:1;text-align:center;padding:12px 8px;background:rgba(248,113,113,0.05);border-radius:8px;border:1px solid rgba(248,113,113,0.14);">
        <div style="font-size:26px;font-weight:700;color:var(--danger);font-family:'Playfair Display',serif;">{{.OrgasmDenied}}</div>
        <div style="font-size:11px;color:var(--text-muted);margin-top:2px;">negados</div>
      </div>
      <div style="flex:1;text-align:center;padding:12px 8px;background:rgba(232,119,154,0.05);border-radius:8px;border:1px solid rgba(232,119,154,0.14);">
        <div style="font-size:26px;font-weight:700;color:var(--pink);font-family:'Playfair Display',serif;">{{.OrgasmTotal}}</div>
        <div style="font-size:11px;color:var(--text-muted);margin-top:2px;">total</div>
      </div>
    </div>
    {{else}}
    <div class="empty">
      <div class="empty-icon">🌸</div>
      <div class="empty-text">Sin solicitudes todavía</div>
    </div>
    {{end}}
  </div>
</div>

{{if .LockStartISO}}
<script>
(function(){
  var wrap = document.getElementById('cu-wrap');
  if (!wrap) return;
  var startDate = new Date(wrap.dataset.start);
  function pad(n){ return String(n).padStart(2,'0'); }
  function tick(){
    var elapsed = Math.max(0, new Date() - startDate);
    var d = Math.floor(elapsed / 86400000);
    var h = Math.floor((elapsed % 86400000) / 3600000);
    var m = Math.floor((elapsed % 3600000) / 60000);
    var s = Math.floor((elapsed % 60000) / 1000);
    document.getElementById('cu-d').textContent = d;
    document.getElementById('cu-h').textContent = pad(h);
    document.getElementById('cu-m').textContent = pad(m);
    document.getElementById('cu-s').textContent = pad(s);
  }
  tick();
  setInterval(tick, 1000);
})();
</script>
{{end}}

{{if .HasEndDate}}
<script>
(function(){
  var wrap = document.getElementById('cd-wrap');
  if (!wrap) return;
  var endISO = wrap.dataset.end;
  var startISO = wrap.dataset.start;
  if (!endISO) return;

  var endDate = new Date(endISO);
  var startDate = startISO ? new Date(startISO) : null;
  var prog = document.getElementById('lock-prog');

  function pad(n){ return String(n).padStart(2,'0'); }

  function tick(){
    var now = new Date();
    var rem = endDate - now;
    if (rem <= 0) {
      wrap.innerHTML = '<span style="color:var(--success);font-family:\'Playfair Display\',serif;font-size:22px;">¡Tiempo cumplido!</span>';
      return;
    }
    var d = Math.floor(rem / 86400000);
    var h = Math.floor((rem % 86400000) / 3600000);
    var m = Math.floor((rem % 3600000) / 60000);
    var s = Math.floor((rem % 60000) / 1000);
    document.getElementById('cd-d').textContent = d;
    document.getElementById('cd-h').textContent = pad(h);
    document.getElementById('cd-m').textContent = pad(m);
    document.getElementById('cd-s').textContent = pad(s);

    if (prog && startDate) {
      var total = endDate - startDate;
      var elapsed = now - startDate;
      var pct = Math.min(100, Math.max(0, elapsed * 100 / total));
      prog.style.width = pct.toFixed(2) + '%';
    }
  }
  tick();
  setInterval(tick, 1000);
})();
</script>
{{end}}
{{end}}`

// ── Calendar ───────────────────────────────────────────────────────────────

var calendarHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Calendario</h1>
  <p class="page-sub">Tu historial de castidad día a día</p>
</div>

<div class="cal-nav">
  <a href="{{.PrevURL}}" class="cal-btn">← anterior</a>
  <span class="cal-month-title">{{.MonthStr}}</span>
  <a href="{{.NextURL}}" class="cal-btn">siguiente →</a>
</div>

<div class="cal-legend">
  <span><span class="legend-swatch" style="background:rgba(232,119,154,0.3);border:1px solid rgba(232,119,154,0.5);"></span>Encerrada</span>
  <span><span class="legend-swatch" style="background:rgba(134,239,172,0.3);border:1px solid rgba(134,239,172,0.5);"></span>Tarea completada</span>
  <span><span class="legend-swatch" style="background:rgba(248,113,113,0.3);border:1px solid rgba(248,113,113,0.5);"></span>Tarea fallida</span>
  <span><span class="legend-swatch" style="border:2px solid var(--warning);background:transparent;"></span>Hoy</span>
  <span style="margin-left:8px;"><span class="ci ci-done ci-task">✓</span> tarea hecha</span>
  <span><span class="ci ci-fail ci-task">✗</span> tarea fallida</span>
  <span><span class="ci ci-pend ci-task">…</span> pendiente</span>
  <span><span class="ci ci-og">🌸</span> orgasmo concedido</span>
  <span><span class="ci ci-od">💧</span> orgasmo negado</span>
</div>

<div class="cal-grid">
  <div class="cal-day-hd">Lun</div>
  <div class="cal-day-hd">Mar</div>
  <div class="cal-day-hd">Mié</div>
  <div class="cal-day-hd">Jue</div>
  <div class="cal-day-hd">Vie</div>
  <div class="cal-day-hd">Sáb</div>
  <div class="cal-day-hd">Dom</div>

  {{range .Weeks}}{{range .}}
  {{if eq .Day 0}}
  <div class="cal-cell cal-empty"></div>
  {{else}}
  {{$cls := "cal-free"}}
  {{if eq .TaskStatus "completed"}}{{$cls = "cal-done"}}{{end}}
  {{if eq .TaskStatus "failed"}}{{$cls = "cal-failed"}}{{end}}
  {{if and .IsLocked (eq .TaskStatus "")}}{{$cls = "cal-locked"}}{{end}}
  {{if and .IsLocked (eq .TaskStatus "pending")}}{{$cls = "cal-locked"}}{{end}}
  <div class="cal-cell {{$cls}}{{if .IsToday}} cal-today{{end}}">
    <div class="cal-head">
      <span class="cal-num">{{.Day}}</span>
      {{if .HoursLocked}}<span class="cal-hours">{{.HoursLocked}}h</span>{{end}}
    </div>
    <div class="cal-body"></div>
    <div class="cal-foot">
      {{if eq .TaskStatus "completed"}}<span class="ci ci-task ci-done" title="Tarea completada">✓</span>{{end}}
      {{if eq .TaskStatus "failed"}}<span class="ci ci-task ci-fail" title="Tarea fallida">✗</span>{{end}}
      {{if eq .TaskStatus "pending"}}<span class="ci ci-task ci-pend" title="Tarea pendiente">…</span>{{end}}
      {{if .OrgasmGranted}}<span class="ci ci-og" title="{{.OrgasmGranted}} orgasmo(s) concedido(s)">🌸{{if gt .OrgasmGranted 1}}<span class="ci-count">{{.OrgasmGranted}}</span>{{end}}</span>{{end}}
      {{if .OrgasmDenied}}<span class="ci ci-od" title="{{.OrgasmDenied}} orgasmo(s) negado(s)">💧{{if gt .OrgasmDenied 1}}<span class="ci-count">{{.OrgasmDenied}}</span>{{end}}</span>{{end}}
    </div>
  </div>
  {{end}}
  {{end}}{{end}}
</div>
{{end}}`

// ── Tasks ──────────────────────────────────────────────────────────────────

var tasksHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Órdenes</h1>
  <p class="page-sub">Historial completo de tareas asignadas</p>
</div>

<div class="stats-grid g3" style="margin-bottom:22px;">
  <div class="stat-card">
    <div class="stat-lbl">Completadas</div>
    <div class="stat-val c-green">{{.Completed}}</div>
    <div class="stat-sub">de {{.Total}} totales</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Fallidas</div>
    <div class="stat-val c-red">{{.Failed}}</div>
    <div class="stat-sub">de {{.Total}} totales</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Pendientes</div>
    <div class="stat-val c-yellow">{{.Pending}}</div>
    <div class="stat-sub">en curso</div>
  </div>
</div>

<!-- Filtros -->
<div style="display:flex; gap:6px; flex-wrap:wrap; margin-bottom:18px;">
  <button class="filter-btn active" data-status="all">Todas</button>
  <button class="filter-btn" data-status="completed">✅ Completadas</button>
  <button class="filter-btn" data-status="failed">💀 Fallidas</button>
  <button class="filter-btn" data-status="pending">⏳ Pendientes</button>
</div>

<style>
.task-card { transition: opacity .15s; display: flex; gap: 16px; align-items: flex-start; }
.task-card.hidden { display: none !important; }

/* Lightbox */
#lb-overlay {
  display: none; position: fixed; inset: 0; background: rgba(0,0,0,.85);
  z-index: 1000; align-items: center; justify-content: center; cursor: zoom-out;
}
#lb-overlay.open { display: flex; }
#lb-overlay img { max-width: 90vw; max-height: 90vh; border-radius: 8px; box-shadow: 0 8px 40px rgba(0,0,0,.6); }
</style>

{{if .Tasks}}
<div id="task-list" style="display:flex; flex-direction:column; gap:10px;">
  {{range .Tasks}}
  <div class="card task-card" data-status="{{.Status}}" style="padding:16px;">

    <!-- Icono de estado -->
    <div style="font-size:22px; flex-shrink:0; margin-top:2px;">
      {{if eq .Status "completed"}}✅{{else if eq .Status "failed"}}💀{{else}}⏳{{end}}
    </div>

    <!-- Contenido -->
    <div style="flex:1; min-width:0;">
      <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap; margin-bottom:6px;">
        <span style="font-size:12px; color:var(--text-muted);">{{formatDate .AssignedAt}}</span>
        {{statusBadge .Status}}
        {{if eq .Status "completed"}}
          {{if .RewardHours}}<span class="badge" style="background:rgba(134,239,172,.12); color:var(--success); border:1px solid rgba(134,239,172,.25);">−{{.RewardHours}}h</span>{{end}}
        {{else if eq .Status "failed"}}
          {{if .PenaltyHours}}<span class="badge" style="background:rgba(248,113,113,.12); color:var(--danger); border:1px solid rgba(248,113,113,.25);">+{{.PenaltyHours}}h</span>{{end}}
        {{end}}
      </div>
      <p style="color:var(--text); line-height:1.6; font-size:13.5px;">{{.Description}}</p>
      {{if .CompletedAt}}
      <p style="font-size:11px; color:var(--text-muted); margin-top:5px;">Completada: {{formatDateTimePtr .CompletedAt}}</p>
      {{end}}
    </div>

    <!-- Foto (si existe) -->
    {{if .PhotoURL}}
    <img src="{{safeURL .PhotoURL}}" alt="Evidencia"
      style="width:72px; height:72px; object-fit:cover; border-radius:8px; border:1px solid var(--border); cursor:zoom-in; flex-shrink:0;"
      onclick="openLightbox(this.src)">
    {{end}}

  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">📋</div>
    <div class="empty-text">Aún no hay tareas registradas</div>
  </div>
</div>
{{end}}

<!-- Lightbox -->
<div id="lb-overlay" onclick="closeLightbox()">
  <img id="lb-img" src="" alt="Evidencia">
</div>

<script>
function openLightbox(src) {
  document.getElementById('lb-img').src = src;
  document.getElementById('lb-overlay').classList.add('open');
}
function closeLightbox() {
  document.getElementById('lb-overlay').classList.remove('open');
}
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeLightbox(); });

document.querySelectorAll('.filter-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const status = btn.dataset.status;
    document.querySelectorAll('.task-card').forEach(card => {
      card.classList.toggle('hidden', status !== 'all' && card.dataset.status !== status);
    });
  });
});
</script>
{{end}}`

var chataskHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Tareas de Comunidad</h1>
  <p class="page-sub">Historial de tareas verificadas por Chaster</p>
</div>

<div class="stats-grid g3" style="margin-bottom:22px;">
  <div class="stat-card">
    <div class="stat-lbl">Aprobadas</div>
    <div class="stat-val c-green">{{.Verified}}</div>
    <div class="stat-sub">de {{.Total}} totales</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Rechazadas</div>
    <div class="stat-val c-red">{{.Rejected}}</div>
    <div class="stat-sub">incluye timeout y abandonadas</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Pendientes</div>
    <div class="stat-val c-yellow">{{.Pending}}</div>
    <div class="stat-sub">en votación</div>
  </div>
</div>

<div style="display:flex; gap:6px; flex-wrap:wrap; margin-bottom:18px;">
  <button class="filter-btn active" data-result="all">Todas</button>
  <button class="filter-btn" data-result="verified">✅ Aprobadas</button>
  <button class="filter-btn" data-result="rejected">❌ Rechazadas</button>
  <button class="filter-btn" data-result="abandoned">💀 Abandonadas</button>
  <button class="filter-btn" data-result="timeout">⏰ Timeout</button>
  <button class="filter-btn" data-result="pending">⏳ Pendientes</button>
</div>

<style>
.ct-card.hidden { display: none !important; }
#lb-overlay { display:none; position:fixed; inset:0; background:rgba(0,0,0,.85); z-index:1000; align-items:center; justify-content:center; cursor:zoom-out; }
#lb-overlay.open { display:flex; }
#lb-overlay img { max-width:90vw; max-height:90vh; border-radius:8px; }
</style>

{{if .Tasks}}
<div id="ct-list" style="display:flex; flex-direction:column; gap:10px;">
  {{range .Tasks}}
  <div class="card ct-card" data-result="{{.Result}}" style="padding:16px; display:flex; gap:16px; align-items:flex-start;">

    <div style="font-size:22px; flex-shrink:0; margin-top:2px;">
      {{if eq .Result "verified"}}✅
      {{else if eq .Result "rejected"}}❌
      {{else if eq .Result "abandoned"}}💀
      {{else if eq .Result "timeout"}}⏰
      {{else}}⏳{{end}}
    </div>

    <div style="flex:1; min-width:0;">
      <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap; margin-bottom:6px;">
        <span style="font-size:12px; color:var(--text-muted);">{{formatDate .AssignedAt}}</span>
        {{if eq .Result "verified"}}<span class="badge badge-success">Aprobada</span>
        {{else if eq .Result "rejected"}}<span class="badge badge-danger">Rechazada</span>
        {{else if eq .Result "abandoned"}}<span class="badge badge-danger">Abandonada</span>
        {{else if eq .Result "timeout"}}<span class="badge badge-muted">Timeout</span>
        {{else}}<span class="badge badge-warning">Pendiente</span>{{end}}
        {{if .ResolvedAt}}<span style="font-size:11px; color:var(--text-muted);">→ {{formatDateTimePtr .ResolvedAt}}</span>{{end}}
      </div>
      <p style="color:var(--text); font-size:13.5px; line-height:1.6;">{{.Description}}</p>
    </div>

    {{if .PhotoURL}}
    <img src="{{safeURL .PhotoURL}}" alt="Evidencia"
      style="width:72px; height:72px; object-fit:cover; border-radius:8px; border:1px solid var(--border); cursor:zoom-in; flex-shrink:0;"
      onclick="openLightbox(this.src)">
    {{end}}

  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">🌐</div>
    <div class="empty-text">Aún no hay tareas comunitarias registradas</div>
    <div class="empty-sub">Usa <code>/chatask</code> en Telegram para asignar una</div>
  </div>
</div>
{{end}}

<div id="lb-overlay" onclick="closeLightbox()"><img id="lb-img" src="" alt=""></div>

<script>
function openLightbox(src) { document.getElementById('lb-img').src=src; document.getElementById('lb-overlay').classList.add('open'); }
function closeLightbox() { document.getElementById('lb-overlay').classList.remove('open'); }
document.addEventListener('keydown', e => { if(e.key==='Escape') closeLightbox(); });
document.querySelectorAll('.filter-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const result = btn.dataset.result;
    document.querySelectorAll('.ct-card').forEach(card => {
      card.classList.toggle('hidden', result !== 'all' && card.dataset.result !== result);
    });
  });
});
</script>
{{end}}`
// ── Orgasms ────────────────────────────────────────────────────────────────

var orgasmsHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Permisos</h1>
  <p class="page-sub">Cada vez que pediste permiso a Papi</p>
</div>

<div class="stats-grid g4" style="margin-bottom:22px;">
  <div class="stat-card">
    <div class="stat-lbl">Total solicitados</div>
    <div class="stat-val c-pink">{{.Total}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Concedidos</div>
    <div class="stat-val c-green">{{.Granted}}</div>
    <div class="stat-sub">{{.GrantPct}}% aprobación</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Juguetes</div>
    <div class="stat-val c-yellow">{{.GrantedToys}}</div>
    <div class="stat-sub">{{if .Total}}{{percent .GrantedToys .Total}}% del total{{end}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Negados</div>
    <div class="stat-val c-red">{{.Denied}}</div>
    <div class="stat-sub">{{if .Total}}{{percent .Denied .Total}}% denegación{{end}}</div>
  </div>
</div>

{{if .Total}}
<div style="margin-bottom:22px;">
  <div style="display:flex;justify-content:space-between;font-size:12px;color:var(--text-muted);margin-bottom:6px;">
    <span>Tasa de concesión global</span>
    <span>{{.GrantPct}}%</span>
  </div>
  <div class="prog-bar" style="height:8px;">
    <div class="prog-fill prog-green" style="{{pctStyle .Granted .Total}}"></div>
  </div>
</div>
{{end}}

{{if .Entries}}
<div class="timeline">
  {{range .Entries}}
  <div class="tl-item {{if or (eq .Outcome "granted_cum") (eq .Outcome "granted")}}tl-granted{{else if eq .Outcome "granted_toys"}}{{else}}tl-denied{{end}}" {{if eq .Outcome "granted_toys"}}style="border-color:rgba(251,191,36,0.2);background:rgba(251,191,36,0.03);"{{end}}>
    <div class="tl-icon">{{if or (eq .Outcome "granted_cum") (eq .Outcome "granted")}}✅{{else if eq .Outcome "granted_toys"}}🧸{{else if eq .Outcome "punished"}}⚠️{{else}}❌{{end}}</div>
    <div class="tl-body">
      <div class="tl-hd">
        <span class="tl-title">{{if or (eq .Outcome "granted_cum") (eq .Outcome "granted")}}Concedido{{else if eq .Outcome "granted_toys"}}Juguetes{{else if eq .Outcome "punished"}}Castigada{{else}}Negado{{end}}</span>
        {{if or (eq .Outcome "granted_cum") (eq .Outcome "granted")}}<span class="badge badge-success">sí</span>{{else if eq .Outcome "granted_toys"}}<span class="badge badge-warning">juguetes</span>{{else if eq .Outcome "punished"}}<span class="badge badge-danger">castigo</span>{{else}}<span class="badge badge-danger">no</span>{{end}}
        <span class="tl-date">{{formatDateTime .CreatedAt}}</span>
      </div>
      {{if .UserMessage}}
      <div class="tl-msg">"{{truncate .UserMessage 120}}"</div>
      {{end}}
      {{if .SenorResponse}}
      <div class="tl-resp">{{truncate .SenorResponse 160}}</div>
      {{end}}
      <div class="tl-meta">
        <span>📅 Día {{.DaysLocked}} de castidad</span>
        {{if .StreakAtTime}}<span>🔥 Racha: {{.StreakAtTime}}</span>{{end}}
        {{if .Condition}}<span>📌 {{truncate .Condition 60}}</span>{{end}}
      </div>
    </div>
  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">🌸</div>
    <div class="empty-text">Aún no has pedido ningún permiso</div>
  </div>
</div>
{{end}}
{{end}}`

// ── Toys ───────────────────────────────────────────────────────────────────

var toysHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Inventario</h1>
  <p class="page-sub">Todos tus juguetes y accesorios</p>
</div>

{{if .Toys}}
<div class="toy-grid">
  {{range .Toys}}
  <div class="toy-card {{if .InUse}}in-use{{end}}">
    {{if .PhotoURL}}
    <img class="toy-img" src="{{safeURL .PhotoURL}}" alt="{{.Name}}" loading="lazy">
    {{else}}
    <div class="toy-placeholder">{{typeIcon .Type}}</div>
    {{end}}
    <div class="toy-info">
      <div class="toy-name">{{.Name}}</div>
      {{if .Description}}<div class="toy-desc">{{truncate .Description 80}}</div>{{end}}
    </div>
    <div class="toy-foot">
      <span class="badge badge-muted">{{typeIcon .Type}} {{typeLabel .Type}}</span>
      {{if .InUse}}<span class="badge badge-pink">en uso</span>{{end}}
    </div>
  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">🎀</div>
    <div class="empty-text">No hay juguetes registrados todavía</div>
  </div>
</div>
{{end}}
{{end}}`

var wardrobeHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Guardarropa</h1>
  <p class="page-sub">Prendas registradas y outfit del día</p>
</div>

{{if .HasTodayOutfit}}
<div class="card" style="border-color:var(--pink); margin-bottom:24px;">
  <div style="display:flex; align-items:center; gap:10px; margin-bottom:14px;">
    <div class="card-title" style="font-size:16px; margin:0;">👗 Outfit de hoy</div>
    {{if .OutfitConfirmed}}
    <span class="badge badge-success">✅ confirmado</span>
    {{else}}
    <span class="badge badge-warning">⏳ esperando foto</span>
    {{end}}
  </div>
  <div style="display:flex; gap:20px; flex-wrap:wrap; align-items:flex-start;">
    {{if and .OutfitConfirmed .TodayPhotoURL}}
    <img src="{{safeURL .TodayPhotoURL}}" alt="Outfit del día"
      style="width:160px; height:220px; object-fit:cover; border-radius:10px; border:1px solid var(--border); flex-shrink:0;">
    {{end}}
    <div style="flex:1; min-width:200px;">
      <p style="color:var(--text); line-height:1.6; margin-bottom:10px;">{{.TodayOutfit}}</p>
      {{if .TodayPose}}
      <p style="color:var(--text-muted); font-size:13px; margin-bottom:10px;">
        <span style="color:var(--pink);">🧍 Pose:</span> {{.TodayPose}}
      </p>
      {{end}}
      {{if and .OutfitConfirmed .TodayComment}}
      <div style="border-top:1px solid var(--border); padding-top:10px;">
        <p style="font-size:11px; color:var(--text-muted); margin-bottom:4px; text-transform:uppercase; letter-spacing:.05em;">Comentario de Papi</p>
        <p style="color:var(--purple); font-style:italic; line-height:1.6;">{{.TodayComment}}</p>
      </div>
      {{end}}
    </div>
  </div>
</div>
{{end}}

<!-- Inventario con filtro -->
<div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:16px; flex-wrap:wrap; gap:10px;">
  <h2 style="font-family:'Playfair Display',serif; font-size:18px; color:var(--text);">Inventario</h2>
  <div id="filter-btns" style="display:flex; gap:6px; flex-wrap:wrap;">
    <button class="filter-btn active" data-type="all">Todo</button>
    <button class="filter-btn" data-type="thong">🩲 Tanga</button>
    <button class="filter-btn" data-type="bra">👙 Sujetador</button>
    <button class="filter-btn" data-type="stockings">🦵 Medias</button>
    <button class="filter-btn" data-type="socks">🧦 Calcetines</button>
    <button class="filter-btn" data-type="collar">💎 Collar</button>
    <button class="filter-btn" data-type="lingerie">🌸 Lencería</button>
    <button class="filter-btn" data-type="dress">👗 Vestido</button>
    <button class="filter-btn" data-type="top">👚 Top</button>
    <button class="filter-btn" data-type="bottom">👘 Falda</button>
    <button class="filter-btn" data-type="shoes">👠 Zapatos</button>
    <button class="filter-btn" data-type="accessory">💍 Accesorio</button>
    <button class="filter-btn" data-type="other">🎀 Otro</button>
  </div>
</div>

<style>
.clothing-item { transition: opacity .2s; }
.clothing-item.hidden { display: none; }
</style>

{{if .Items}}
<div class="toy-grid" id="clothing-grid">
  {{range .Items}}
  <div class="toy-card clothing-item" data-type="{{.Type}}">
    {{if .PhotoURL}}
    <img class="toy-img" src="{{safeURL .PhotoURL}}" alt="{{.Name}}" loading="lazy">
    {{else}}
    <div class="toy-placeholder">{{clothingIcon .Type}}</div>
    {{end}}
    <div class="toy-info">
      <div class="toy-name">{{.Name}}</div>
      {{if .Description}}<div class="toy-desc">{{truncate .Description 80}}</div>{{end}}
    </div>
    <div class="toy-foot">
      <span class="badge badge-muted">{{clothingIcon .Type}} {{clothingLabel .Type}}</span>
    </div>
  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">👗</div>
    <div class="empty-text">No hay prendas registradas todavía</div>
    <div class="empty-sub">Usa <code>/wardrobe add</code> en Telegram para añadir</div>
  </div>
</div>
{{end}}

<!-- Historial de outfits -->
{{if .History}}
<h2 style="font-family:'Playfair Display',serif; font-size:18px; color:var(--text); margin:32px 0 16px;">Historial de outfits</h2>
<div style="display:flex; flex-direction:column; gap:14px;">
  {{range .History}}
  <div class="card" style="padding:16px;">
    <div style="display:flex; gap:16px; align-items:flex-start; flex-wrap:wrap;">
      {{if .PhotoURL}}
      <img src="{{safeURL .PhotoURL}}" alt="Outfit {{.Date}}"
        style="width:90px; height:120px; object-fit:cover; border-radius:8px; border:1px solid var(--border); flex-shrink:0;">
      {{else}}
      <div style="width:90px; height:120px; border-radius:8px; background:var(--sidebar); border:1px solid var(--border); display:flex; align-items:center; justify-content:center; font-size:28px; flex-shrink:0;">👗</div>
      {{end}}
      <div style="flex:1; min-width:160px;">
        <p style="font-size:12px; color:var(--text-muted); margin-bottom:6px;">{{.Date}}</p>
        <p style="color:var(--text); line-height:1.5; font-size:13px; margin-bottom:6px;">{{.OutfitDesc}}</p>
        {{if .PoseDesc}}
        <p style="font-size:12px; color:var(--text-muted); margin-bottom:6px;"><span style="color:var(--pink);">🧍</span> {{.PoseDesc}}</p>
        {{end}}
        {{if .Comment}}
        <p style="font-size:12px; color:var(--purple); font-style:italic; line-height:1.5;">&#8220;{{.Comment}}&#8221;</p>
        {{end}}
      </div>
    </div>
  </div>
  {{end}}
</div>
{{end}}

<script>
document.querySelectorAll('.filter-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const type = btn.dataset.type;
    document.querySelectorAll('.clothing-item').forEach(item => {
      item.classList.toggle('hidden', type !== 'all' && item.dataset.type !== type);
    });
  });
});
</script>
{{end}}`

var galleryHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Galería</h1>
  <p class="page-sub">{{.Total}} fotos en total</p>
</div>

<div style="display:flex; gap:6px; flex-wrap:wrap; margin-bottom:20px;">
  <button class="gal-btn active" data-cat="all">Todas ({{.Total}})</button>
  <button class="gal-btn" data-cat="task">📋 Tareas</button>
  <button class="gal-btn" data-cat="outfit">👗 Outfits</button>
  <button class="gal-btn" data-cat="toy">🎀 Juguetes</button>
  <button class="gal-btn" data-cat="clothing">🩲 Ropa</button>
  <button class="gal-btn" data-cat="chatask">🌐 Comunidad</button>
</div>

<style>
.gal-btn { background:var(--card); border:1px solid var(--border); color:var(--text-muted); padding:5px 14px; border-radius:20px; cursor:pointer; font-size:12px; font-family:'Inter',sans-serif; transition:all .15s; }
.gal-btn:hover { border-color:var(--pink); color:var(--pink); }
.gal-btn.active { background:rgba(232,119,154,.15); border-color:var(--pink); color:var(--pink); }
.gal-grid { columns: 4 160px; column-gap: 12px; }
.gal-item { break-inside: avoid; margin-bottom: 12px; border-radius: 10px; overflow: hidden; border: 1px solid var(--border); cursor: zoom-in; position: relative; transition: border-color .15s; background: var(--card); }
.gal-item:hover { border-color: var(--pink); }
.gal-item.hidden { display: none; }
.gal-img { width: 100%; display: block; object-fit: cover; }
.gal-overlay { position: absolute; bottom: 0; left: 0; right: 0; background: linear-gradient(transparent, rgba(0,0,0,.75)); padding: 24px 10px 10px; opacity: 0; transition: opacity .2s; }
.gal-item:hover .gal-overlay { opacity: 1; }
.gal-cap { font-size: 11px; color: #fff; line-height: 1.4; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.gal-date { font-size: 10px; color: rgba(255,255,255,.6); margin-top: 3px; }
.gal-cat-badge { position: absolute; top: 8px; right: 8px; font-size: 14px; }
#lb2-overlay { display:none; position:fixed; inset:0; background:rgba(0,0,0,.9); z-index:1000; align-items:center; justify-content:center; cursor:zoom-out; flex-direction:column; gap:12px; }
#lb2-overlay.open { display:flex; }
#lb2-overlay img { max-width:92vw; max-height:82vh; border-radius:8px; object-fit:contain; }
#lb2-cap { color: rgba(255,255,255,.8); font-size: 13px; max-width: 500px; text-align:center; }
</style>

{{if .Photos}}
<div class="gal-grid" id="gal-grid">
  {{range .Photos}}
  <div class="gal-item" data-cat="{{.Category}}" data-cap="{{truncate .Caption 80}}" data-date="{{formatDate .Date}}">
    <img class="gal-img" src="{{safeURL .URL}}" alt="{{.Caption}}" loading="lazy">
    <div class="gal-cat-badge">{{if eq .Category "task"}}📋{{else if eq .Category "outfit"}}👗{{else if eq .Category "toy"}}🎀{{else if eq .Category "clothing"}}🩲{{else}}🌐{{end}}</div>
    <div class="gal-overlay">
      <div class="gal-cap">{{truncate .Caption 60}}</div>
      <div class="gal-date">{{formatDate .Date}}</div>
    </div>
  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">🖼️</div>
    <div class="empty-text">Aún no hay fotos registradas</div>
  </div>
</div>
{{end}}

<div id="lb2-overlay" onclick="closeLB()">
  <img id="lb2-img" src="" alt="">
  <div id="lb2-cap"></div>
</div>

<script>
function closeLB() { document.getElementById('lb2-overlay').classList.remove('open'); }
document.addEventListener('keydown', e => { if(e.key==='Escape') closeLB(); });

document.querySelectorAll('.gal-item').forEach(item => {
  item.addEventListener('click', function() {
    var img = this.querySelector('img');
    document.getElementById('lb2-img').src = img ? img.src : '';
    var cap = this.dataset.cap || '';
    var date = this.dataset.date || '';
    document.getElementById('lb2-cap').textContent = cap + (date ? ' · ' + date : '');
    document.getElementById('lb2-overlay').classList.add('open');
  });
});

document.querySelectorAll('.gal-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.gal-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const cat = btn.dataset.cat;
    document.querySelectorAll('.gal-item').forEach(item => {
      item.classList.toggle('hidden', cat !== 'all' && item.dataset.cat !== cat);
    });
  });
});
</script>
{{end}}`

var contractHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Contrato</h1>
  <p class="page-sub">Las reglas que Papi impuso para esta sesión</p>
</div>

{{if .HasContract}}
<div class="card" style="border-color: rgba(232,119,154,0.4); max-width: 720px;">
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:20px; flex-wrap:wrap; gap:8px;">
    <span style="font-family:'Playfair Display',serif; font-size:18px; color:var(--pink);">📜 Contrato activo</span>
    <span style="font-size:12px; color:var(--text-muted);">{{formatDateTime .Contract.CreatedAt}}</span>
  </div>
  <div style="white-space: pre-wrap; line-height: 1.8; color: var(--text); font-size: 14px; font-style: italic; border-left: 2px solid rgba(232,119,154,0.4); padding-left: 18px;">{{.Contract.Text}}</div>
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">📜</div>
    <div class="empty-text">No hay contrato activo</div>
    <div style="font-size:12px; color:var(--text-muted); margin-top:8px;">Se genera automáticamente al iniciar una nueva sesión</div>
  </div>
</div>
{{end}}
{{end}}`

var checkinsHTML = `{{define "content"}}
<div class="page-hd">
  <h1 class="page-title">Check-ins</h1>
  <p class="page-sub">Historial de verificaciones espontáneas de Papi</p>
</div>

<div class="stats-grid g4" style="margin-bottom:22px;">
  <div class="stat-card">
    <div class="stat-lbl">Total solicitados</div>
    <div class="stat-val c-pink">{{.Total}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Aprobados</div>
    <div class="stat-val c-green">{{.Approved}}</div>
    <div class="stat-sub">{{.ResponseRate}}% respuesta</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Fallidos</div>
    <div class="stat-val c-red">{{.Missed}}</div>
    <div class="stat-sub">ignorados o rechazados</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Tiempo medio</div>
    <div class="stat-val c-purple">{{.AvgResponse}}<span style="font-size:16px;">min</span></div>
    <div class="stat-sub">respuesta promedio</div>
  </div>
</div>

{{if .Total}}
<div style="margin-bottom:22px;">
  <div style="display:flex;justify-content:space-between;font-size:12px;color:var(--text-muted);margin-bottom:6px;">
    <span>Tasa de respuesta</span>
    <span>{{.ResponseRate}}%</span>
  </div>
  <div class="prog-bar" style="height:8px;">
    <div class="prog-fill prog-green" style="{{pctStyle .Approved .Total}}"></div>
  </div>
</div>
{{end}}

<style>
#lb3-overlay { display:none; position:fixed; inset:0; background:rgba(0,0,0,.9); z-index:1000; align-items:center; justify-content:center; cursor:zoom-out; }
#lb3-overlay.open { display:flex; }
#lb3-overlay img { max-width:90vw; max-height:90vh; border-radius:8px; }
</style>
<script>
document.addEventListener('keydown', function(e){ if(e.key==='Escape'){ document.getElementById('lb3-overlay').classList.remove('open'); } });
</script>

{{if .Entries}}
<div style="display:flex; flex-direction:column; gap:10px;">
  {{range .Entries}}
  <div class="card" style="padding:16px; display:flex; gap:16px; align-items:flex-start;">

    <div style="font-size:22px; flex-shrink:0; margin-top:2px;">
      {{if or (eq .Status "submitted") (eq .Status "approved")}}✅
      {{else if eq .Status "missed"}}⏰
      {{else if eq .Status "rejected"}}❌
      {{else}}⏳{{end}}
    </div>

    <div style="flex:1; min-width:0;">
      <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap; margin-bottom:6px;">
        <span style="font-weight:600; font-size:13px;">
          {{if or (eq .Status "submitted") (eq .Status "approved")}}Enviado
          {{else if eq .Status "missed"}}No respondido
          {{else if eq .Status "rejected"}}Rechazado
          {{else}}Pendiente{{end}}
        </span>
        {{if or (eq .Status "submitted") (eq .Status "approved")}}<span class="badge badge-success">✓</span>
        {{else if eq .Status "missed"}}<span class="badge badge-warning">timeout</span>
        {{else if eq .Status "rejected"}}<span class="badge badge-danger">rechazado</span>
        {{else}}<span class="badge badge-muted">esperando</span>{{end}}
        <span style="font-size:11px; color:var(--text-muted); margin-left:auto;">{{formatDateTime .RequestedAt}}</span>
      </div>
      <div style="display:flex; gap:16px; font-size:12px; color:var(--text-muted); flex-wrap:wrap;">
        {{if .RespondedAt}}<span>Respondido: {{formatDateTimePtr .RespondedAt}}</span>{{end}}
        {{if and (or (eq .Status "submitted") (eq .Status "approved")) .ResponseTimeMins}}<span>⏱ {{.ResponseTimeMins}} min de respuesta</span>{{end}}
      </div>
    </div>

    {{if .PhotoURL}}
    <img src="{{safeURL .PhotoURL}}" alt="Check-in"
      style="width:72px; height:72px; object-fit:cover; border-radius:8px; border:1px solid var(--border); cursor:zoom-in; flex-shrink:0;"
      onclick="document.getElementById('lb3-img').src=this.src; document.getElementById('lb3-overlay').classList.add('open')">
    {{end}}

  </div>
  {{end}}
</div>
{{else}}
<div class="card">
  <div class="empty">
    <div class="empty-icon">📸</div>
    <div class="empty-text">Aún no hay check-ins registrados</div>
    <div style="font-size:12px; color:var(--text-muted); margin-top:8px;">Papi los enviará aleatoriamente durante el día</div>
  </div>
</div>
{{end}}

<div id="lb3-overlay" onclick="this.classList.remove('open')">
  <img id="lb3-img" src="" alt="Check-in">
</div>
{{end}}`
