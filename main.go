package main

import (
	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/scheduler"
	"chaster-keyholder/storage"
	"chaster-keyholder/telegram"
	"chaster-keyholder/tts"
	"chaster-keyholder/web"
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

	// Google Cloud TTS (opcional)
	ttsKey := os.Getenv("GOOGLE_TTS_API_KEY")
	ttsVoice := os.Getenv("GOOGLE_TTS_VOICE") // ej. "es-US-Neural2-B"
	if ttsKey != "" {
		bot.SetTTS(tts.New(ttsKey, ttsVoice))
		log.Println("✅ Google Cloud TTS configurado")
	} else {
		log.Println("⚠️  Google Cloud TTS no configurado — sin notas de voz")
	}

	log.Println("🔒 Chaster Keyholder Bot iniciado")

	go scheduler.Start(bot)
	dashPassword := os.Getenv("DASHBOARD_PASSWORD")
	go startWebServer(db, botUsername, dashPassword)

	bot.Start()
}

func startWebServer(db *storage.DB, botUsername, dashPassword string) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := web.New(db, "state.json", botUsername, dashPassword)
	log.Printf("🌐 Dashboard iniciado en puerto %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Printf("error en servidor web: %v", err)
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Variable de entorno requerida no encontrada: %s", key)
	}
	return val
}
