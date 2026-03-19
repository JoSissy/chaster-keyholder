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

	// Ritual matutino — 8:30 AM
	s.NewJob(
		gocron.CronJob("30 8 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Iniciando ritual matutino...")
			bot.StartMorningRitual()
		}),
	)

	// Asignación de plug — 8:45 AM
	s.NewJob(
		gocron.CronJob("45 8 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Asignando plug del día...")
			bot.SendPlugAssignment()
		}),
	)

	// Mensajes de condicionamiento — 10am y 2pm
	s.NewJob(
		gocron.CronJob("0 10,14 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando mensaje de condicionamiento...")
			bot.SendConditioningMessage()
		}),
	)

	// Check-ins espontáneos — 11am y 3pm
	s.NewJob(
		gocron.CronJob("0 11,15 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Disparando check-in...")
			bot.TriggerCheckin()
		}),
	)

	// Ruleta diaria — 6pm
	s.NewJob(
		gocron.CronJob("0 18 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Girando ruleta diaria...")
			bot.HandleRuleta()
		}),
	)

	// Verificar expiración de check-ins — cada 5 minutos
	s.NewJob(
		gocron.CronJob("*/5 * * * *", false),
		gocron.NewTask(func() {
			bot.CheckCheckinExpiry()
		}),
	)

	// Juicio dominical — domingos a las 9pm
	s.NewJob(
		gocron.CronJob("0 21 * * 0", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Ejecutando juicio dominical...")
			bot.HandleWeeklyJudgment()
		}),
	)

	s.Start()
	log.Println("✅ Scheduler iniciado")
}
