package scheduler

import (
	"log"

	"github.com/go-co-op/gocron/v2"

	"chaster-keyholder/telegram"
)

func Start(bot *telegram.Bot) {
	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("error creando scheduler:", err)
	}

	// Status matutino — 8:00 AM
	s.NewJob(
		gocron.CronJob("0 8 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando status matutino...")
			bot.SendMorningStatus()
		}),
	)

	// Status nocturno — 10:00 PM
	s.NewJob(
		gocron.CronJob("0 22 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando status nocturno...")
			bot.SendNightStatus()
		}),
	)

	// Tarea diaria — 9:00 AM
	s.NewJob(
		gocron.CronJob("0 9 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Asignando tarea diaria...")
			bot.HandleTask()
		}),
	)

	// Check si el lock terminó — cada minuto
	s.NewJob(
		gocron.CronJob("* * * * *", false),
		gocron.NewTask(func() {
			bot.CheckLockFinished()
		}),
	)

	// Eventos random — cada 30 minutos en horario activo (8am-11pm COT)
	s.NewJob(
		gocron.CronJob("*/30 8-22 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Evaluando evento random...")
			bot.HandleRandomEvent()
		}),
	)

	// Verificar expiración de eventos activos — cada 5 minutos
	s.NewJob(
		gocron.CronJob("*/5 * * * *", false),
		gocron.NewTask(func() {
			bot.CheckActiveEventExpiry()
		}),
	)

	// Mensajes random del keyholder — cada 4 horas en horario activo (8am-11pm)
	s.NewJob(
		gocron.CronJob("0 8,12,16,20 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando mensaje random...")
			bot.SendRandomMessage()
		}),
	)

	s.Start()
	log.Println("✅ Scheduler iniciado")
}