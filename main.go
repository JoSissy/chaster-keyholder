package main

import (
	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/scheduler"
	"chaster-keyholder/storage"
	"chaster-keyholder/telegram"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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
	go startWebServer(botUsername)

	bot.Start()
}

func startWebServer(botUsername string) {
	telegramLink := "https://t.me/" + botUsername
	if botUsername == "" {
		telegramLink = "#"
	}

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
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ChasterAI Keyholder</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0d0d0d; color: #e0e0e0;
      min-height: 100vh; display: flex;
      align-items: center; justify-content: center; padding: 24px;
    }
    .card {
      background: #161616; border: 1px solid #2a2a2a;
      border-radius: 16px; padding: 40px 32px;
      max-width: 420px; width: 100%; text-align: center;
    }
    .icon { font-size: 48px; margin-bottom: 16px; }
    h1 { font-size: 22px; font-weight: 700; color: #fff; margin-bottom: 8px; }
    .subtitle { font-size: 14px; color: #666; margin-bottom: 32px; line-height: 1.5; }
    .features { text-align: left; margin-bottom: 32px; display: flex; flex-direction: column; gap: 12px; }
    .feature { display: flex; align-items: flex-start; gap: 12px; font-size: 14px; color: #aaa; }
    .feature-icon { font-size: 16px; flex-shrink: 0; margin-top: 1px; }
    .divider { height: 1px; background: #2a2a2a; margin-bottom: 28px; }
    .btn {
      display: inline-flex; align-items: center; justify-content: center;
      gap: 10px; background: #2481cc; color: #fff; text-decoration: none;
      padding: 14px 28px; border-radius: 12px; font-size: 15px;
      font-weight: 600; width: 100%; transition: background 0.2s;
    }
    .btn:hover { background: #1a6eb0; }
    .btn svg { width: 20px; height: 20px; fill: white; flex-shrink: 0; }
    .footer { margin-top: 20px; font-size: 12px; color: #444; }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">🔒</div>
    <h1>ChasterAI Keyholder</h1>
    <p class="subtitle">An AI keyholder that manages your lock, assigns daily tasks, and keeps you under control 24/7.</p>
    <div class="features">
      <div class="feature"><span class="feature-icon">📋</span><span>Daily tasks with photo verification via AI vision</span></div>
      <div class="feature"><span class="feature-icon">❄️</span><span>Random control events — freeze, hide timer, pillory</span></div>
      <div class="feature"><span class="feature-icon">⏱</span><span>Automatic time rewards and penalties</span></div>
      <div class="feature"><span class="feature-icon">💬</span><span>Chat freely with your AI keyholder anytime</span></div>
    </div>
    <div class="divider"></div>
    <a class="btn" href="` + telegramLink + `" target="_blank">
      <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.447 1.394c-.16.16-.295.295-.605.295l.213-3.053 5.56-5.023c.242-.213-.054-.333-.373-.12L8.32 13.617l-2.96-.924c-.64-.203-.658-.64.135-.954l11.566-4.461c.537-.194 1.006.131.833.943z"/>
      </svg>
      Open in Telegram
    </a>
    <p class="footer">Powered by Groq · Built for Chaster</p>
  </div>
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
