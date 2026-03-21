package scheduler

import (
	"log"
	"math/rand"
	"time"

	"github.com/go-co-op/gocron/v2"

	"chaster-keyholder/telegram"
)

// Start registra todos los jobs del scheduler y arranca el loop de check-ins.
// TODOS los horarios son en COT (Colombia, UTC-5). El servidor puede estar en
// cualquier zona horaria — gocron usa la zona del sistema, pero el código en bot.go
// usa cotLocation explícitamente para comparaciones de fecha.
//
// Resumen de jobs:
//   08:00 — status matutino + mensaje de Papi
//   08:30 — ritual matutino (foto de jaula + mensaje escrito)
//   08:45 — asignación de plug del día
//   09:00 — tarea diaria
//   10:00 — outfit del día + mensaje de condicionamiento
//   11:00 — expiración del ritual (penaliza si no se completó)
//   12:00, 16:00, 20:00 — mensajes random de condicionamiento
//   14:00 — mensaje de condicionamiento
//   18:00 — ruleta diaria
//   22:00 — status nocturno + penalización si tarea no completada
//   23:00 — decay de obediencia (si 2+ días sin tarea, ConsecutiveDays = 0)
//   cada 1 min  — check si el lock terminó (CheckLockFinished)
//   cada 5 min  — expiración de eventos activos, check-ins, recordatorio de plug
//   cada 15 min — voto comunitario de tareas de Chaster
//   cada 30 min (8-22h) — eventos random (freeze, hidetime, pillory, etc.)
//   domingos 21:00 — juicio semanal (WeeklyDebt → consecuencias)
//   intervalo aleatorio (45min-3h, 8-23h) — check-ins espontáneos (goroutine separada)
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
			bot.WithLock(bot.SendMorningStatus)
		}),
	)

	// Status nocturno — 10:00 PM
	s.NewJob(
		gocron.CronJob("0 22 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando status nocturno...")
			bot.WithLock(bot.SendNightStatus)
		}),
	)

	// Tarea diaria — 9:00 AM
	s.NewJob(
		gocron.CronJob("0 9 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Asignando tarea diaria...")
			bot.WithLock(bot.HandleTask)
		}),
	)

	// Check si el lock terminó — cada minuto
	s.NewJob(
		gocron.CronJob("* * * * *", false),
		gocron.NewTask(func() {
			bot.WithLock(bot.CheckLockFinished)
		}),
	)

	// Eventos random — cada 30 minutos en horario activo (8am-11pm COT)
	s.NewJob(
		gocron.CronJob("*/30 8-22 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Evaluando evento random...")
			bot.WithLock(bot.HandleRandomEvent)
		}),
	)

	// Verificar expiración de eventos activos y check-ins — cada 5 minutos
	s.NewJob(
		gocron.CronJob("*/5 * * * *", false),
		gocron.NewTask(func() {
			bot.WithLock(func() {
				bot.CheckActiveEventExpiry()
				bot.CheckCheckinExpiry()
				bot.CheckPlugReminder()
			})
		}),
	)

	// Mensajes random del keyholder — cada 4 horas en horario activo (8am-11pm)
	s.NewJob(
		gocron.CronJob("0 8,12,16,20 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando mensaje random...")
			bot.WithLock(bot.SendRandomMessage)
		}),
	)

	// Ritual matutino — 8:30 AM
	s.NewJob(
		gocron.CronJob("30 8 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Iniciando ritual matutino...")
			bot.WithLock(bot.StartMorningRitual)
		}),
	)

	// Asignación de plug — 8:45 AM
	s.NewJob(
		gocron.CronJob("45 8 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Asignando plug del día...")
			bot.WithLock(bot.SendPlugAssignment)
		}),
	)

	// Outfit del día — 10:00 AM
	s.NewJob(
		gocron.CronJob("0 10 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Asignando outfit del día...")
			bot.WithLock(bot.SendDailyOutfit)
		}),
	)

	// Mensajes de condicionamiento — 10am y 2pm
	s.NewJob(
		gocron.CronJob("0 10,14 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Enviando mensaje de condicionamiento...")
			bot.WithLock(bot.SendConditioningMessage)
		}),
	)

	// Ruleta diaria — 6pm
	s.NewJob(
		gocron.CronJob("0 18 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Girando ruleta diaria...")
			bot.WithLock(bot.HandleRuleta)
		}),
	)

	// Penalizar ritual matutino ignorado — 11am
	s.NewJob(
		gocron.CronJob("0 11 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Verificando expiración de ritual matutino...")
			bot.WithLock(bot.CheckRitualExpiry)
		}),
	)

	// Juicio dominical — domingos a las 9pm
	s.NewJob(
		gocron.CronJob("0 21 * * 0", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Ejecutando juicio dominical...")
			bot.WithLock(bot.HandleWeeklyJudgment)
		}),
	)

	// Verificar voto comunitario de tareas de Chaster — cada 15 minutos
	s.NewJob(
		gocron.CronJob("*/15 * * * *", false),
		gocron.NewTask(func() {
			bot.WithLock(bot.CheckChasterTaskVote)
		}),
	)

	// Decay de obediencia por inactividad — 11pm diario
	s.NewJob(
		gocron.CronJob("0 23 * * *", false),
		gocron.NewTask(func() {
			log.Println("[scheduler] Verificando decay de obediencia...")
			bot.WithLock(bot.CheckObedienceDecay)
		}),
	)

	s.Start()
	log.Println("✅ Scheduler iniciado")

	// Loop de check-in con intervalo aleatorio (45min - 3h) en horario 8am-11pm COT
	go runCheckinLoop(bot)
}

// runCheckinLoop ejecuta check-ins en intervalos aleatorios entre 45 minutos y 3 horas,
// solo en horario activo (8am-11pm COT).
func runCheckinLoop(bot *telegram.Bot) {
	// Espera inicial aleatoria para no disparar al arrancar
	initialWait := time.Duration(45+rand.Intn(75)) * time.Minute
	log.Printf("[checkin-loop] primera espera: %v", initialWait)
	time.Sleep(initialWait)

	for {
		loc, _ := time.LoadLocation("America/Bogota")
		hour := time.Now().In(loc).Hour()

		if hour >= 8 && hour < 23 {
			log.Println("[checkin-loop] Evaluando check-in...")
			bot.WithLock(bot.TriggerCheckin)
		}

		// Intervalo aleatorio: 45 minutos a 3 horas
		wait := time.Duration(45+rand.Intn(135)) * time.Minute
		log.Printf("[checkin-loop] próximo check-in en: %v", wait)
		time.Sleep(wait)
	}
}
