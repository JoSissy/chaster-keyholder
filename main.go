package main

import (
	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/scheduler"
	"chaster-keyholder/storage"
	"chaster-keyholder/telegram"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Archivo .env no encontrado, usando variables del sistema")
	}

	chasterToken := mustEnv("CHASTER_TOKEN")
	telegramToken := mustEnv("TELEGRAM_BOT_TOKEN")
	chatIDStr := mustEnv("TELEGRAM_CHAT_ID")
	groqKey := mustEnv("GROQ_API_KEY")
	botUsername := os.Getenv("TELEGRAM_BOT_USERNAME")

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("TELEGRAM_CHAT_ID inválido:", err)
	}

	// Base de datos SQLite
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "keyholder.db"
	}
	db, err := storage.NewDB(dbPath)
	if err != nil {
		log.Fatal("Error iniciando base de datos:", err)
	}

	// Cloudinary
	cloudinaryCloudName := mustEnv("CLOUDINARY_CLOUD_NAME")
	cloudinaryAPIKey := mustEnv("CLOUDINARY_API_KEY")
	cloudinaryAPISecret := mustEnv("CLOUDINARY_API_SECRET")
	cloudinary := storage.NewCloudinaryClient(cloudinaryCloudName, cloudinaryAPIKey, cloudinaryAPISecret)
	log.Println("✅ Cloudinary configurado")

	// Cliente Chaster
	chasterClient := chaster.NewClient(chasterToken)
	extToken := os.Getenv("CHASTER_EXTENSION_TOKEN")
	extSlug := os.Getenv("CHASTER_EXTENSION_SLUG")
	if extToken != "" && extSlug != "" {
		chasterClient.WithExtension(extToken, extSlug)
		log.Println("✅ Extensions API configurada —", extSlug)
	} else {
		log.Println("⚠️  Extensions API no configurada — freeze/pillory/hidetime no disponibles")
	}

	aiClient := ai.NewClient(groqKey)

	bot, err := telegram.NewBot(telegramToken, chatID, chasterClient, aiClient, db, cloudinary)
	if err != nil {
		log.Fatal("Error iniciando bot de Telegram:", err)
	}

	log.Println("🔒 Chaster Keyholder Bot iniciado")

	go scheduler.Start(bot)
	go startWebServer(botUsername, db, chasterClient)

	bot.Start()
}

// ── Status API ─────────────────────────────────────────────────────────────

type statusResp struct {
	Locked bool  `json:"locked"`
	Frozen bool  `json:"frozen"`
	Start  int64 `json:"start"` // unix — inicio del lock
	End    int64 `json:"end"`   // unix — fin estimado (0 = oculto)
	Streak int   `json:"streak"`
	Done   int   `json:"done"`
	Failed int   `json:"failed"`
}

var (
	scMu   sync.Mutex
	scData *statusResp
	scTime time.Time
)

func getStatusCached(db *storage.DB, ch *chaster.Client) *statusResp {
	scMu.Lock()
	defer scMu.Unlock()
	if scData != nil && time.Since(scTime) < 30*time.Second {
		return scData
	}
	r := &statusResp{}
	if lock, err := ch.GetActiveLock(); err == nil {
		r.Locked = true
		r.Frozen = lock.Frozen
		r.Start = lock.StartDate.Unix()
		if lock.EndDate != nil {
			r.End = lock.EndDate.Unix()
		}
	}
	if state, err := db.LoadSessionState(); err == nil {
		r.Streak = state.TasksStreak
		r.Done = state.TasksCompleted
		r.Failed = state.TasksFailed
	}
	scData = r
	scTime = time.Now()
	return r
}

// ── Web server ─────────────────────────────────────────────────────────────

func startWebServer(botUsername string, db *storage.DB, ch *chaster.Client) {
	telegramLink := "https://t.me/" + botUsername
	if botUsername == "" {
		telegramLink = "#"
	}

	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getStatusCached(db, ch))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, buildPage(telegramLink))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🌐 Servidor web iniciado en puerto %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("error en servidor web: %v", err)
	}
}

func buildPage(telegramLink string) string {
	return `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ChasterAI Keyholder</title>
  <style>
    *, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }

    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
      background: #0a0009;
      color: #ddd0d8;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }

    .wrap {
      width: 100%;
      max-width: 400px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    /* ── Status card ── */
    #status {
      background: #0f0b12;
      border: 1px solid #2a1f35;
      border-radius: 18px;
      padding: 28px 24px 24px;
      text-align: center;
      display: none;
      position: relative;
      overflow: hidden;
    }
    #status.locked-glow::before {
      content: '';
      position: absolute;
      top: -40px; left: 50%;
      transform: translateX(-50%);
      width: 200px; height: 120px;
      background: radial-gradient(ellipse, rgba(232,53,109,0.12) 0%, transparent 70%);
      pointer-events: none;
    }
    #status.frozen-glow::before {
      background: radial-gradient(ellipse, rgba(139,96,240,0.12) 0%, transparent 70%);
    }

    .dot {
      width: 7px; height: 7px;
      border-radius: 50%;
      display: inline-block;
      margin-right: 5px;
      vertical-align: middle;
      position: relative;
      top: -1px;
    }
    .dot.pink { background: #e8356d; box-shadow: 0 0 5px #e8356d; animation: blink 2.4s ease-in-out infinite; }
    .dot.grey { background: #3a2d35; }
    .dot.blue { background: #8b60f0; box-shadow: 0 0 5px #8b60f0; animation: blink 2.4s ease-in-out infinite; }

    @keyframes blink {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.3; }
    }

    .badge {
      display: inline-flex;
      align-items: center;
      padding: 5px 14px;
      border-radius: 20px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      margin-bottom: 22px;
    }
    .badge.locked { background: rgba(232,53,109,0.1); border: 1px solid rgba(232,53,109,0.3); color: #e8356d; }
    .badge.frozen { background: rgba(139,96,240,0.1); border: 1px solid rgba(139,96,240,0.3); color: #a87ef5; }
    .badge.free   { background: rgba(60,45,55,0.3);   border: 1px solid rgba(60,45,55,0.6);   color: #4a3845; }

    .countdown {
      font-size: 44px;
      font-weight: 800;
      color: #f5eef2;
      letter-spacing: -0.03em;
      font-variant-numeric: tabular-nums;
      line-height: 1;
      margin-bottom: 7px;
    }
    .countdown.frozen-color { color: #c4a8ff; }
    .countdown.hidden-color { color: #2d2030; }

    .countdown-label {
      font-size: 10px;
      color: #3d2835;
      text-transform: uppercase;
      letter-spacing: 0.12em;
      margin-bottom: 18px;
    }

    .since {
      font-size: 13px;
      color: #5a3f50;
      margin-bottom: 22px;
      line-height: 1.5;
    }
    .since b { color: #9b7080; font-weight: 600; }

    .sep { height: 1px; background: #1e1422; margin-bottom: 18px; }

    .stats { display: flex; justify-content: center; }
    .stat { flex: 1; text-align: center; padding: 0 6px; }
    .stat + .stat { border-left: 1px solid #1e1422; }
    .stat-val {
      font-size: 26px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      line-height: 1;
      margin-bottom: 5px;
    }
    .stat-val.v-streak { color: #e8356d; }
    .stat-val.v-done   { color: #6b4a58; }
    .stat-val.v-failed { color: #3d2530; }
    .stat-label { font-size: 10px; color: #3d2835; text-transform: uppercase; letter-spacing: 0.1em; }

    .free-msg { font-size: 14px; color: #3a2830; padding: 6px 0 2px; }

    /* ── Main card ── */
    .card {
      background: #110d14;
      border: 1px solid #1e1825;
      border-radius: 18px;
      padding: 32px 24px;
      text-align: center;
    }

    .lock-icon { font-size: 34px; margin-bottom: 14px; }
    h1 { font-size: 19px; font-weight: 700; color: #f0e8ed; margin-bottom: 8px; letter-spacing: -0.01em; }
    .subtitle { font-size: 13px; color: #3d2d38; line-height: 1.65; margin-bottom: 28px; }

    .features { text-align: left; margin-bottom: 28px; display: flex; flex-direction: column; gap: 11px; }
    .feature { display: flex; align-items: flex-start; gap: 10px; font-size: 13px; color: #5a4050; }
    .fi { font-size: 13px; flex-shrink: 0; margin-top: 1px; }

    hr { height: 1px; background: #1a1520; border: none; margin-bottom: 24px; }

    .btn {
      display: inline-flex; align-items: center; justify-content: center;
      gap: 10px; background: #2481cc; color: #fff; text-decoration: none;
      padding: 14px 28px; border-radius: 12px; font-size: 14px;
      font-weight: 600; width: 100%; transition: background 0.18s;
    }
    .btn:hover { background: #1a6eb0; }
    .btn svg { width: 18px; height: 18px; fill: white; flex-shrink: 0; }

    .footer { margin-top: 16px; font-size: 11px; color: #261d24; }
  </style>
</head>
<body>
<div class="wrap">

  <div id="status"></div>

  <div class="card">
    <div class="lock-icon">🔒</div>
    <h1>ChasterAI Keyholder</h1>
    <p class="subtitle">An AI keyholder that manages your lock, assigns daily tasks, and keeps you under control 24/7.</p>
    <div class="features">
      <div class="feature"><span class="fi">📋</span><span>Daily tasks with photo verification via AI vision</span></div>
      <div class="feature"><span class="fi">❄️</span><span>Random control events — freeze, hide timer, pillory</span></div>
      <div class="feature"><span class="fi">⏱</span><span>Automatic time rewards and penalties</span></div>
      <div class="feature"><span class="fi">💬</span><span>Chat freely with your AI keyholder anytime</span></div>
    </div>
    <hr>
    <a class="btn" href="` + telegramLink + `" target="_blank">
      <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.447 1.394c-.16.16-.295.295-.605.295l.213-3.053 5.56-5.023c.242-.213-.054-.333-.373-.12L8.32 13.617l-2.96-.924c-.64-.203-.658-.64.135-.954l11.566-4.461c.537-.194 1.006.131.833.943z"/>
      </svg>
      Open in Telegram
    </a>
    <p class="footer">Powered by Groq · Built for Chaster</p>
  </div>

</div>
<script>
  function pad(n) { return String(n).padStart(2, '0'); }

  function fmtRemaining(sec) {
    if (sec <= 0) return '00:00:00';
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    if (d > 0) return d + 'd ' + pad(h) + 'h ' + pad(m) + 'm';
    return pad(h) + ':' + pad(m) + ':' + pad(s);
  }

  function fmtElapsed(sec) {
    if (sec <= 0) return '0m';
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0 && h > 0) return d + 'd ' + h + 'h';
    if (d > 0) return d + 'd';
    if (h > 0) return h + 'h ' + m + 'm';
    return m + 'm';
  }

  let sd = null;

  function renderStatus() {
    const el = document.getElementById('status');
    if (!sd) { el.style.display = 'none'; return; }

    if (!sd.locked) {
      el.className = '';
      el.style.display = 'block';
      el.innerHTML =
        '<span class="badge free"><span class="dot grey"></span>libre</span>' +
        '<div class="free-msg">sin sesión activa</div>';
      return;
    }

    const now = Math.floor(Date.now() / 1000);
    const elapsed = Math.max(0, now - sd.start);
    const remaining = sd.end > 0 ? Math.max(0, sd.end - now) : -1;
    const isFrozen = sd.frozen;

    const badgeClass  = isFrozen ? 'frozen' : 'locked';
    const dotClass    = isFrozen ? 'blue' : 'pink';
    const badgeText   = isFrozen ? '❄ congelada' : 'encerrada';
    const cdClass     = isFrozen ? 'countdown frozen-color' : 'countdown';
    const glowClass   = isFrozen ? 'frozen-glow' : 'locked-glow';

    let cdHTML, labelHTML;
    if (remaining >= 0) {
      cdHTML    = '<div class="' + cdClass + '" id="cdown">' + fmtRemaining(remaining) + '</div>';
      labelHTML = '<div class="countdown-label">tiempo restante</div>';
    } else {
      cdHTML    = '<div class="countdown hidden-color">— : — : —</div>';
      labelHTML = '<div class="countdown-label">temporizador oculto</div>';
    }

    el.className = glowClass;
    el.style.display = 'block';
    el.innerHTML =
      '<span class="badge ' + badgeClass + '"><span class="dot ' + dotClass + '"></span>' + badgeText + '</span>' +
      cdHTML +
      labelHTML +
      '<div class="since">enjauladita hace <b id="celapsed">' + fmtElapsed(elapsed) + '</b></div>' +
      '<div class="sep"></div>' +
      '<div class="stats">' +
        '<div class="stat"><div class="stat-val v-streak">' + sd.streak + '</div><div class="stat-label">racha</div></div>' +
        '<div class="stat"><div class="stat-val v-done">'   + sd.done   + '</div><div class="stat-label">completadas</div></div>' +
        '<div class="stat"><div class="stat-val v-failed">' + sd.failed + '</div><div class="stat-label">fallidas</div></div>' +
      '</div>';
  }

  function tick() {
    if (!sd || !sd.locked) return;
    const now = Math.floor(Date.now() / 1000);

    const cdown = document.getElementById('cdown');
    if (cdown && sd.end > 0) {
      cdown.textContent = fmtRemaining(Math.max(0, sd.end - now));
    }

    const celapsed = document.getElementById('celapsed');
    if (celapsed) {
      celapsed.textContent = fmtElapsed(Math.max(0, now - sd.start));
    }
  }

  async function fetchStatus() {
    try {
      const r = await fetch('/api/status');
      sd = await r.json();
      renderStatus();
    } catch(e) { /* silently fail */ }
  }

  fetchStatus();
  setInterval(fetchStatus, 30000);
  setInterval(tick, 1000);
</script>
</body>
</html>`
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Variable de entorno requerida no encontrada: %s", key)
	}
	return val
}
