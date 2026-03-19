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

	chasterClient := chaster.NewClient(chasterToken)
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