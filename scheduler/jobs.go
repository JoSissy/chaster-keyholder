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

	s.Start()
	log.Println("✅ Scheduler iniciado")
}
