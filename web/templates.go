package web

var baseHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Jolie's Diary</title>
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
</style>
</head>
<body>

<aside class="sidebar">
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
    <a href="/orgasms" class="nav-link {{if eq .Nav "orgasms"}}active{{end}}">
      <span class="nav-icon">🌸</span> Permisos
    </a>
    <a href="/toys" class="nav-link {{if eq .Nav "toys"}}active{{end}}">
      <span class="nav-icon">🎀</span> Inventario
    </a>
    <a href="/wardrobe" class="nav-link {{if eq .Nav "wardrobe"}}active{{end}}">
      <span class="nav-icon">👗</span> Guardarropa
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
  <div class="lock-info" style="flex:1;">

    <div style="display:flex;align-items:baseline;gap:16px;flex-wrap:wrap;">
      <div>
        <div class="lock-days">{{.DaysLocked}}</div>
        <div class="lock-lbl">días encerrada</div>
      </div>
      {{if .HasEndDate}}
      <div id="cd-wrap" style="display:flex;align-items:flex-end;gap:6px;" data-end="{{.LockEndISO}}" data-start="{{.LockStartISO}}">
        <div class="cd-unit"><span class="cd-num" id="cd-d">—</span><span class="cd-lbl">días</span></div>
        <span class="cd-sep">:</span>
        <div class="cd-unit"><span class="cd-num" id="cd-h">——</span><span class="cd-lbl">horas</span></div>
        <span class="cd-sep">:</span>
        <div class="cd-unit"><span class="cd-num" id="cd-m">——</span><span class="cd-lbl">min</span></div>
        <span class="cd-sep">:</span>
        <div class="cd-unit"><span class="cd-num" id="cd-s">——</span><span class="cd-lbl">seg</span></div>
      </div>
      {{end}}
    </div>

    {{if .HasEndDate}}
    <div style="margin:14px 0 10px;">
      <div style="display:flex;justify-content:space-between;font-size:11px;color:var(--text-muted);margin-bottom:5px;">
        <span>{{formatDatePtr .LockStartDate}}</span>
        <span>{{.ProgressPct}}% completado</span>
        <span>{{formatDatePtr .LockEndDate}}</span>
      </div>
      <div class="prog-bar" style="height:6px;" id="lock-prog-bar">
        <div class="prog-fill" id="lock-prog" style="{{pctStyle .ProgressPct 100}}"></div>
      </div>
    </div>
    {{end}}

    <div class="lock-badges">
      <span class="badge badge-pink">🔥 Racha: {{.Streak}}</span>
      <span class="badge badge-purple">Obediencia {{.ObedienceName}}</span>
      {{if .WeeklyDebt}}<span class="badge badge-danger">Deuda: {{.WeeklyDebt}}h</span>{{end}}
      {{if .PendingCheckin}}<span class="badge badge-warning">⚠️ Check-in pendiente</span>{{end}}
    </div>
  </div>
</div>
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

<div class="stats-grid g4" style="margin-top:-8px;">
  <div class="stat-card">
    <div class="stat-lbl">Permisos concedidos</div>
    <div class="stat-val c-green">{{.OrgasmGranted}}</div>
    <div class="stat-sub">de {{.OrgasmTotal}} solicitados</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Permisos negados</div>
    <div class="stat-val c-red">{{.OrgasmDenied}}</div>
    <div class="stat-sub">{{.GrantRate}}% aprobación</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Tiempo añadido</div>
    <div class="stat-val c-red">+{{.TimeAdded}}h</div>
    <div class="stat-sub">como castigo</div>
  </div>
  <div class="stat-card">
    <div class="stat-lbl">Tiempo quitado</div>
    <div class="stat-val c-green">−{{.TimeRemoved}}h</div>
    <div class="stat-sub">como recompensa</div>
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
.filter-btn {
  background: var(--card); border: 1px solid var(--border);
  color: var(--text-muted); padding: 5px 14px; border-radius: 20px;
  cursor: pointer; font-size: 12px; font-family: 'Inter', sans-serif; transition: all .15s;
}
.filter-btn:hover { border-color: var(--pink); color: var(--pink); }
.filter-btn.active { background: rgba(232,119,154,.15); border-color: var(--pink); color: var(--pink); }
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
.filter-btn { background:var(--card); border:1px solid var(--border); color:var(--text-muted); padding:5px 14px; border-radius:20px; cursor:pointer; font-size:12px; font-family:'Inter',sans-serif; transition:all .15s; }
.filter-btn:hover { border-color:var(--pink); color:var(--pink); }
.filter-btn.active { background:rgba(232,119,154,.15); border-color:var(--pink); color:var(--pink); }
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

<div class="stats-grid g3" style="margin-bottom:22px;">
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
  <div class="tl-item {{if .Granted}}tl-granted{{else}}tl-denied{{end}}">
    <div class="tl-icon">{{if .Granted}}✅{{else}}❌{{end}}</div>
    <div class="tl-body">
      <div class="tl-hd">
        <span class="tl-title">{{if .Granted}}Concedido{{else}}Negado{{end}}</span>
        {{if .Granted}}<span class="badge badge-success">sí</span>{{else}}<span class="badge badge-danger">no</span>{{end}}
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
.filter-btn {
  background: var(--card);
  border: 1px solid var(--border);
  color: var(--text-muted);
  padding: 5px 12px;
  border-radius: 20px;
  cursor: pointer;
  font-size: 12px;
  font-family: 'Inter', sans-serif;
  transition: all .15s;
}
.filter-btn:hover { border-color: var(--pink); color: var(--pink); }
.filter-btn.active { background: rgba(232,119,154,.15); border-color: var(--pink); color: var(--pink); }
.clothing-item { transition: opacity .2s, transform .2s; }
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
