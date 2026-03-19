package main

import (
	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/scheduler"
	"chaster-keyholder/telegram"
	"log"
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

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("TELEGRAM_CHAT_ID inválido:", err)
	}

	// Cliente base (Public API)
	chasterClient := chaster.NewClient(chasterToken)

	// Extensión — opcional. Si las variables están presentes, activa freeze/pillory/etc.
	extToken := os.Getenv("CHASTER_EXTENSION_TOKEN")
	extSlug := os.Getenv("CHASTER_EXTENSION_SLUG")
	if extToken != "" && extSlug != "" {
		chasterClient.WithExtension(extToken, extSlug)
		log.Println("✅ Extensions API configurada —", extSlug)
	} else {
		log.Println("⚠️  Extensions API no configurada — freeze/pillory/hidetime no disponibles")
		log.Println("   Añade CHASTER_EXTENSION_TOKEN y CHASTER_EXTENSION_SLUG al .env para activarlos")
	}

	aiClient := ai.NewClient(groqKey)

	bot, err := telegram.NewBot(telegramToken, chatID, chasterClient, aiClient)
	if err != nil {
		log.Fatal("Error iniciando bot de Telegram:", err)
	}

	log.Println("🔒 Chaster Keyholder Bot iniciado")

	go scheduler.Start(bot)

	bot.Start()
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Variable de entorno requerida no encontrada: %s", key)
	}
	return val
}