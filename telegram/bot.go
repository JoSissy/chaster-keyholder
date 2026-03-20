package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/models"
	"chaster-keyholder/storage"
)

type Bot struct {
	api        *tgbotapi.BotAPI
	chatID     int64
	chaster    *chaster.Client
	ai         *ai.Client
	state      *models.AppState
	statePath  string
	db         *storage.DB
	cloudinary *storage.CloudinaryClient

	// Rate limiting
	lastChatTime time.Time
	chatMu       sync.Mutex

	// Protección de escrituras concurrentes al estado
	stateMu sync.Mutex

	// Caché de días encerrada (evita llamadas repetidas a la API)
	cachedDaysLocked   int
	cachedDaysLockedAt time.Time

	// Estado de UI transitorio — no se persiste entre reinicios
	pendingAction string
}

func NewBot(token string, chatID int64, chasterClient *chaster.Client, aiClient *ai.Client, db *storage.DB, cloudinary *storage.CloudinaryClient) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	b := &Bot{
		api:        api,
		chatID:     chatID,
		chaster:    chasterClient,
		ai:         aiClient,
		statePath:  "state.json",
		db:         db,
		cloudinary: cloudinary,
	}
	b.state = b.loadState() // usa b.db internamente como fallback
	// Cargar juguetes desde DB
	if toys, err := db.GetToys(); err == nil {
		b.state.Toys = make([]models.Toy, 0, len(toys))
		for _, t := range toys {
			b.state.Toys = append(b.state.Toys, storageToyToModel(t))
		}
	}
	// Restaurar pendingAction desde el estado para recuperación tras reinicio
	switch {
	case b.state.RitualStep == 1:
		b.pendingAction = "ritual_photo"
	case b.state.RitualStep == 2:
		b.pendingAction = "ritual_message"
	case b.state.PendingCheckin && b.state.CheckinExpiresAt != nil && time.Now().Before(*b.state.CheckinExpiresAt):
		b.pendingAction = "checkin_photo"
	case b.state.AssignedPlugID != "" && !b.state.PlugConfirmed && b.state.AssignedPlugDate == todayStr():
		b.pendingAction = "plug_photo"
	case b.state.PendingChasterTask != "":
		b.pendingAction = "chaster_task_photo"
	case b.state.DailyOutfitDesc != "" && !b.state.OutfitConfirmed && b.state.DailyOutfitDate == todayStr():
		b.pendingAction = "outfit_photo"
	}
	return b, nil
}

// ── Estado ─────────────────────────────────────────────────────────────────

func (b *Bot) loadState() *models.AppState {
	data, err := os.ReadFile(b.statePath)
	if err != nil {
		return b.loadStateFromDB()
	}
	var s models.AppState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("error parseando state.json: %v — intentando DB", err)
		return b.loadStateFromDB()
	}
	if s.Toys == nil {
		s.Toys = []models.Toy{}
	}
	// Si el state.json existe pero los contadores están todos en cero,
	// restaurar los campos críticos desde la DB por si acaso
	if s.TasksStreak == 0 && s.TasksCompleted == 0 && b.db != nil {
		if ss, err := b.db.LoadSessionState(); err == nil {
			s.TasksStreak = ss.TasksStreak
			s.TasksCompleted = ss.TasksCompleted
			s.TasksFailed = ss.TasksFailed
			s.TotalTimeAddedHours = ss.TotalTimeAddedHours
			s.TotalTimeRemovedHours = ss.TotalTimeRemovedHours
			s.WeeklyDebt = ss.WeeklyDebt
			s.WeeklyDebtDetails = ss.WeeklyDebtDetails
			s.LastJudgmentDate = ss.LastJudgmentDate
			if s.CurrentLockID == "" {
				s.CurrentLockID = ss.CurrentLockID
			}
			log.Println("✅ Contadores restaurados desde DB")
		}
	}
	return &s
}

func (b *Bot) loadStateFromDB() *models.AppState {
	s := &models.AppState{Toys: []models.Toy{}}
	if b.db == nil {
		return s
	}
	ss, err := b.db.LoadSessionState()
	if err != nil {
		log.Printf("no se encontró session_state en DB: %v", err)
		return s
	}
	s.TasksStreak = ss.TasksStreak
	s.TasksCompleted = ss.TasksCompleted
	s.TasksFailed = ss.TasksFailed
	s.TotalTimeAddedHours = ss.TotalTimeAddedHours
	s.TotalTimeRemovedHours = ss.TotalTimeRemovedHours
	s.WeeklyDebt = ss.WeeklyDebt
	s.WeeklyDebtDetails = ss.WeeklyDebtDetails
	s.LastJudgmentDate = ss.LastJudgmentDate
	s.CurrentLockID = ss.CurrentLockID
	log.Println("✅ Estado restaurado desde DB (state.json no disponible)")
	return s
}

// saveState guarda el estado usando write atómico para evitar corrupción
func (b *Bot) saveState() error {
	data, err := json.MarshalIndent(b.state, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando estado: %w", err)
	}

	tmp := b.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo estado temporal: %w", err)
	}

	if err := os.Rename(tmp, b.statePath); err != nil {
		return fmt.Errorf("error aplicando estado: %w", err)
	}

	return nil
}

func (b *Bot) mustSaveState() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	if err := b.saveState(); err != nil {
		log.Printf("CRÍTICO — error guardando estado: %v", err)
	}
	if b.db != nil {
		if err := b.db.SaveSessionState(&storage.SessionState{
			TasksStreak:           b.state.TasksStreak,
			TasksCompleted:        b.state.TasksCompleted,
			TasksFailed:           b.state.TasksFailed,
			TotalTimeAddedHours:   b.state.TotalTimeAddedHours,
			TotalTimeRemovedHours: b.state.TotalTimeRemovedHours,
			WeeklyDebt:            b.state.WeeklyDebt,
			WeeklyDebtDetails:     b.state.WeeklyDebtDetails,
			LastJudgmentDate:      b.state.LastJudgmentDate,
			CurrentLockID:         b.state.CurrentLockID,
		}); err != nil {
			log.Printf("error guardando session_state en DB: %v", err)
		}
	}
}

// ── Mensajes ───────────────────────────────────────────────────────────────

func (b *Bot) Send(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("error enviando mensaje con Markdown: %v — reintentando sin formato", err)
		plain := tgbotapi.NewMessage(b.chatID, stripMarkdown(text))
		b.api.Send(plain)
	}
}

func stripMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"*", "",
		"_", "",
		"`", "",
		"[", "",
		"]", "",
	)
	return replacer.Replace(s)
}

func (b *Bot) SendWithKeyboard(text string, buttons [][]tgbotapi.KeyboardButton) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.ReplyKeyboardMarkup{
		Keyboard:       buttons,
		ResizeKeyboard: true,
	}
	b.api.Send(msg)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (b *Bot) daysLocked() int {
	// Caché de 5 minutos — evita llamadas repetidas a la API de Chaster
	b.stateMu.Lock()
	if !b.cachedDaysLockedAt.IsZero() && time.Since(b.cachedDaysLockedAt) < 5*time.Minute {
		cached := b.cachedDaysLocked
		b.stateMu.Unlock()
		return cached
	}
	b.stateMu.Unlock()

	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return b.state.DaysLocked
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24

	b.stateMu.Lock()
	b.cachedDaysLocked = days
	b.cachedDaysLockedAt = time.Now()
	b.state.DaysLocked = days
	b.stateMu.Unlock()

	return days
}

func (b *Bot) downloadFile(fileID string) ([]byte, string, error) {
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.api.Token, file.FilePath)
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	mime := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(file.FilePath), ".png") {
		mime = "image/png"
	} else if strings.HasSuffix(strings.ToLower(file.FilePath), ".webp") {
		mime = "image/webp"
	}

	return data, mime, nil
}

func (b *Bot) deleteMessage(messageID int) {
	del := tgbotapi.NewDeleteMessage(b.chatID, messageID)
	b.api.Request(del)
}

// ── Comandos ───────────────────────────────────────────────────────────────

func (b *Bot) HandleStatus() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No se encontró sesión activa en Chaster.")
		return
	}

	// Si el lock está listo para desbloquear, ejecutarlo automáticamente y silenciosamente
	if lock.IsReadyToUnlock {
		b.finishLock(lock.ID)
		return
	}

	var elapsed time.Duration
	if !lock.StartDate.IsZero() {
		elapsed = time.Since(lock.StartDate)
	} else {
		elapsed = time.Duration(lock.TotalDuration) * time.Second
	}
	days := int(elapsed.Hours()) / 24
	hours := int(elapsed.Hours()) % 24
	mins := int(elapsed.Minutes()) % 60

	var timeRemaining string
	if lock.EndDate != nil {
		remaining := time.Until(*lock.EndDate)
		if remaining > 0 {
			timeRemaining = chaster.FormatDuration(int64(remaining.Seconds()))
		} else {
			timeRemaining = "¡tiempo cumplido!"
		}
	} else {
		timeRemaining = "indefinido"
	}

	intensity := models.GetIntensity(days)

	// ── Estado del lock ──
	stateLines := ""
	if lock.Frozen {
		stateLines += "\n❄️ *CONGELADA*"
	}
	if b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt) {
		mins := int(time.Until(b.state.ActiveEvent.ExpiresAt).Minutes())
		switch b.state.ActiveEvent.Type {
		case "hidetime":
			stateLines += fmt.Sprintf("\n🙈 Timer oculto — *%d min*", mins)
		}
	}

	// ── Tarea diaria ──
	taskStatus := "sin asignar"
	if b.state.CurrentTask != nil {
		if b.state.CurrentTask.Completed {
			taskStatus = "✅ completada"
		} else if b.state.CurrentTask.Failed {
			taskStatus = "💀 fallida"
		} else if b.state.CurrentTask.AwaitingPhoto {
			taskStatus = "📸 _esperando foto..._"
		} else {
			taskStatus = fmt.Sprintf("⏳ _%s_", b.state.CurrentTask.Description)
		}
	}

	// ── Tarea comunitaria ──
	chasterTaskLine := ""
	if b.state.PendingChasterTask != "" {
		chasterTaskLine = "\n🌐 Tarea comunidad — 📸 _esperando foto..._"
	} else if b.state.ChasterTaskLockID != "" {
		chasterTaskLine = "\n🌐 Tarea comunidad — ⏳ _votando..._"
	}

	// ── Plug del día ──
	plugLine := ""
	if b.state.AssignedPlugID != "" && b.state.AssignedPlugDate == todayStr() {
		plugName := b.getAssignedPlugName()
		if b.state.PlugConfirmed {
			plugLine = fmt.Sprintf("\n🔌 Plug — *%s* ✅", plugName)
		} else {
			plugLine = fmt.Sprintf("\n🔌 Plug — *%s* ⏳", plugName)
		}
	}

	// ── Check-in ──
	checkinLine := ""
	if b.state.PendingCheckin && b.state.CheckinExpiresAt != nil && time.Now().Before(*b.state.CheckinExpiresAt) {
		minsLeft := int(time.Until(*b.state.CheckinExpiresAt).Minutes())
		checkinLine = fmt.Sprintf("\n📸 Check-in pendiente — *%d min*", minsLeft)
	}

	// ── Obediencia y orgasmo ──
	orgasmStatus := "bloqueado"
	if b.state.TasksStreak >= 9 {
		orgasmStatus = "puede concederse"
	} else if b.state.TasksStreak >= 4 {
		orgasmStatus = "difícil"
	}
	daysSinceOrgasm := -1
	if b.db != nil {
		daysSinceOrgasm = b.db.GetDaysSinceLastOrgasm()
	}
	orgasmDaysLine := ""
	if daysSinceOrgasm < 0 {
		orgasmDaysLine = " — nunca"
	} else {
		orgasmDaysLine = fmt.Sprintf(" — *%d días*", daysSinceOrgasm)
	}

	// ── Ruleta ──
	ruletaLine := ""
	if b.state.LastRuletaDate != todayStr() {
		ruletaLine = "\n🎰 Ruleta — _disponible_"
	}

	// ── Deuda semanal ──
	debtLine := ""
	if b.state.WeeklyDebt > 0 {
		debtLine = fmt.Sprintf("\n⚠️ Deuda semanal — *%d infracciones*", b.state.WeeklyDebt)
	}

	msg := fmt.Sprintf(
		"▪️ *ESTADO DE CONDENA*\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"⏱ Encerrada — *%dd %dh %dm*\n"+
			"⌛ Restante — *%s*\n"+
			"🌡 Nivel — *%s*%s\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"📋 Tarea — %s%s\n"+
			"✅ Completadas — *%d* | 💀 Fallidas — *%d*\n"+
			"🔥 Racha — *%d* tareas | Obediencia — *%s*%s%s\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"💦 Orgasmo — _%s_%s\n"+
			"📊 Balance — *+%dh / -%dh*%s%s",
		days, hours, mins,
		timeRemaining,
		intensity.String(), stateLines,
		taskStatus, chasterTaskLine,
		b.state.TasksCompleted, b.state.TasksFailed,
		b.state.TasksStreak, models.ObedienceTitle(b.state.TasksStreak), plugLine, checkinLine,
		orgasmStatus, orgasmDaysLine,
		b.state.TotalTimeAddedHours, b.state.TotalTimeRemovedHours,
		ruletaLine, debtLine,
	)
	b.Send(msg)
}

func (b *Bot) HandleTask() {
	b.handleTaskInternal(models.IntensityLevel(0))
}

func (b *Bot) HandleTaskWithLevel(args string) {
	args = strings.ToLower(strings.TrimSpace(args))
	var level models.IntensityLevel
	switch args {
	case "light", "suave", "1":
		level = models.IntensityLight
	case "moderate", "moderada", "medium", "2":
		level = models.IntensityModerate
	case "intense", "intensa", "hard", "3":
		level = models.IntensityIntense
	case "max", "maximum", "maxima", "máxima", "extreme", "4":
		level = models.IntensityMaximum
	default:
		b.Send("▪️ *NIVELES DISPONIBLES*\n▬▬▬▬▬▬▬▬▬▬▬▬\n`/order light` — suave\n`/order moderate` — moderada\n`/order intense` — intensa\n`/order max` — máxima")
		return
	}
	b.handleTaskInternal(level)
}

func (b *Bot) handleTaskInternal(forcedLevel models.IntensityLevel) {
	if b.state.CurrentTask != nil && !b.state.CurrentTask.Completed && !b.state.CurrentTask.Failed {
		awaiting := ""
		if b.state.CurrentTask.AwaitingPhoto {
			awaiting = "\n\n_Manda la foto cuando estés lista._"
		}
		b.Send(fmt.Sprintf(
			"▪️ *ORDEN ACTIVA*\n"+
				"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
				"_%s_\n"+
				"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
				"⏰ Límite — *%s*\n"+
				"💀 Consecuencia — *+%dh*%s",
			b.state.CurrentTask.Description,
			b.state.CurrentTask.DueAt.Format("15:04"),
			b.state.CurrentTask.PenaltyHours,
			awaiting,
		))
		return
	}

	days := b.daysLocked()
	var intensity models.IntensityLevel
	if forcedLevel == 0 {
		intensity = models.GetIntensity(days)
	} else {
		intensity = forcedLevel
	}

	var recentTasks []string
	if b.db != nil {
		recentTasks, _ = b.db.GetRecentTaskDescriptions(10)
	}
	taskDesc, err := b.ai.GenerateDailyTask(days, b.state.Toys, intensity, recentTasks)
	if err != nil {
		b.Send("❌ Error generando tarea.")
		return
	}

	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("COT", -5*60*60)
	}
	now := time.Now().In(loc)

	penaltyHours := 1 + int(intensity)

	b.state.CurrentTask = &models.Task{
		ID:            fmt.Sprintf("task-%d", now.Unix()),
		Description:   taskDesc,
		AssignedAt:    now,
		DueAt:         now.Add(1 * time.Hour),
		PenaltyHours:  penaltyHours,
		RewardHours:   0,
		AwaitingPhoto: true,
	}
	b.mustSaveState()

	b.Send(fmt.Sprintf(
		"▪️ *NUEVA ORDEN* — nivel %s\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"_%s_\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"⏰ Límite — *%s*\n"+
			"💀 Consecuencia — *+%dh*\n\n"+
			"_Manda la foto cuando termines._",
		intensity.String(),
		taskDesc,
		b.state.CurrentTask.DueAt.Format("15:04"),
		penaltyHours,
	))
}

func (b *Bot) HandlePhoto(imageBytes []byte, mimeType string) {
	if b.state.CurrentTask == nil || b.state.CurrentTask.Completed || b.state.CurrentTask.Failed {
		b.Send("No hay tarea activa esperando evidencia.")
		return
	}

	if !b.state.CurrentTask.AwaitingPhoto {
		b.Send("No estoy esperando una foto ahora.")
		return
	}

	// Subir foto a Cloudinary
	var photoURL string
	if b.cloudinary != nil {
		url, _, cerr := b.cloudinary.Upload(imageBytes, mimeType, "chaster/tasks")
		if cerr != nil {
			log.Printf("error subiendo foto de tarea: %v", cerr)
		} else {
			photoURL = url
		}
	}

	b.state.CurrentTask.Completed = true
	b.state.CurrentTask.AwaitingPhoto = false
	b.state.TasksCompleted++

	// Puntos: +2 si está en intensidad alta (8+ días), +1 normal
	taskPoints := 1
	if b.daysLocked() >= 8 {
		taskPoints = 2
	}
	prevTitle := models.ObedienceTitle(b.state.TasksStreak)
	b.state.TasksStreak += taskPoints

	// Días consecutivos y bono de 7 días
	b.state.ConsecutiveDays++
	if b.state.ConsecutiveDays%7 == 0 {
		b.state.TasksStreak += 3
	}
	b.state.LastTaskCompletedDate = todayStr()

	newStreak := b.state.TasksStreak
	b.mustSaveState()
	defer b.checkStreakMilestone(prevTitle, newStreak)

	// Guardar en DB
	if b.db != nil {
		now := time.Now()
		b.db.SaveTask(&storage.Task{
			ID: b.state.CurrentTask.ID, LockID: b.state.CurrentLockID,
			Description: b.state.CurrentTask.Description, PhotoURL: photoURL,
			AssignedAt: b.state.CurrentTask.AssignedAt, DueAt: b.state.CurrentTask.DueAt,
			CompletedAt: &now, Status: "completed",
			PenaltyHours: b.state.CurrentTask.PenaltyHours, RewardHours: 0,
		})
	}

	aiMsg, _ := b.ai.GenerateTaskAccepted(b.state.Toys, b.daysLocked())
	b.Send("✅ *EVIDENCIA RECIBIDA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + aiMsg)
}

func (b *Bot) HandleFail() {
	if b.state.CurrentTask == nil || b.state.CurrentTask.Completed || b.state.CurrentTask.Failed {
		b.Send("No hay tarea pendiente.")
		return
	}

	penaltyHours := b.state.CurrentTask.PenaltyHours
	b.state.CurrentTask.Failed = true
	b.state.CurrentTask.AwaitingPhoto = false
	b.state.TotalTimeAddedHours += penaltyHours
	b.state.TasksFailed++
	b.state.TasksStreak = max(0, b.state.TasksStreak-3)
	b.mustSaveState()

	if lock, err := b.chaster.GetActiveLock(); err == nil {
		if err := b.chaster.AddTime(lock.ID, penaltyHours*3600); err != nil {
			log.Printf("error añadiendo tiempo en Chaster: %v", err)
		}
	}

	if b.db != nil {
		b.db.SaveTask(&storage.Task{
			ID: b.state.CurrentTask.ID, LockID: b.state.CurrentLockID,
			Description: b.state.CurrentTask.Description,
			AssignedAt:  b.state.CurrentTask.AssignedAt, DueAt: b.state.CurrentTask.DueAt,
			Status:       "failed",
			PenaltyHours: penaltyHours, RewardHours: b.state.CurrentTask.RewardHours,
		})
	}

	msg, _ := b.ai.GenerateTaskPenalty(penaltyHours, "confesó que no pudo completar la tarea")
	b.Send("▪️ *TAREA ABANDONADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + msg + fmt.Sprintf("\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas.", penaltyHours))
	b.addWeeklyDebt("tarea abandonada voluntariamente")
	b.autoPillory("confesó que no pudo completar la tarea")
}

// ── Freeze / Timer visibility ──────────────────────────────────────────────
// Todas estas funciones obtienen el lock activo automáticamente.

func (b *Bot) HandleFreeze() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	if err := b.chaster.FreezeLock(lock.ID); err != nil {
		b.Send(fmt.Sprintf("❌ Error congelando el lock: %v", err))
		return
	}
	b.Send("❄️ *LOCK CONGELADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_El tiempo está detenido. No puedes hacer nada._")
}

func (b *Bot) HandleUnfreeze() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	if err := b.chaster.UnfreezeLock(lock.ID); err != nil {
		b.Send(fmt.Sprintf("❌ Error descongelando el lock: %v", err))
		return
	}
	b.Send("🔥 *LOCK DESCONGELADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_El tiempo sigue corriendo. Sin descanso._")
}

func (b *Bot) HandleHideTime() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	if err := b.chaster.SetTimerVisibility(lock.ID, false); err != nil {
		b.Send(fmt.Sprintf("❌ Error ocultando el tiempo: %v", err))
		return
	}
	b.Send("🙈 *TIEMPO OCULTO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Ya no sabes cuánto te queda. Así me gusta._")
}

func (b *Bot) HandleShowTime() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	if err := b.chaster.SetTimerVisibility(lock.ID, true); err != nil {
		b.Send(fmt.Sprintf("❌ Error mostrando el tiempo: %v", err))
		return
	}
	b.Send("👁 *TIEMPO VISIBLE*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Puedes ver cuánto te queda. Sufre con eso._")
}

func (b *Bot) HandlePillory(durationMinutes int, reason string) {
	if durationMinutes < 5 {
		durationMinutes = 5
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	// Generar razón en inglés para la comunidad de Chaster
	engReason, err := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, reason)
	if err != nil || strings.TrimSpace(engReason) == "" {
		engReason = reason
	}
	engReason = strings.TrimSpace(engReason)
	if err := b.chaster.PutInPillory(lock.ID, durationMinutes*60, engReason); err != nil {
		b.Send(fmt.Sprintf("❌ Error enviando al cepo: %v", err))
		return
	}
	b.Send(fmt.Sprintf(
		"⛓ *ENVIADA AL CEPO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n▬▬▬▬▬▬▬▬▬▬▬▬\nDuración — *%d min*",
		reason, durationMinutes,
	))
}

// handleToyRemoveSelection procesa la selección de juguete a eliminar
func (b *Bot) handleToyRemoveSelection(text string) {
	var num int
	fmt.Sscanf(strings.TrimSpace(text), "%d", &num)

	if num < 1 || num > len(b.state.Toys) {
		lines := []string{"❌ Número inválido. ¿Cuál quieres eliminar?"}
		for i, t := range b.state.Toys {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, t.Name))
		}
		b.Send(strings.Join(lines, "\n"))
		return
	}

	selected := b.state.Toys[num-1]

	if b.db != nil {
		b.db.DeleteToy(selected.ID)
	}

	// Borrar imagen de Cloudinary si existe
	if b.cloudinary != nil && selected.PhotoPublicID != "" {
		if err := b.cloudinary.Delete(selected.PhotoPublicID); err != nil {
			log.Printf("error borrando imagen de Cloudinary (%s): %v", selected.PhotoPublicID, err)
		}
	}

	newToys := []models.Toy{}
	for _, t := range b.state.Toys {
		if t.ID != selected.ID {
			newToys = append(newToys, t)
		}
	}
	b.state.Toys = newToys
	b.pendingAction = ""
	b.mustSaveState()

	b.Send(fmt.Sprintf("🗑 *%s* eliminado.", selected.Name))
}

// handleCageSelection procesa la selección de jaula durante el flujo de newlock
func (b *Bot) handleCageSelection(text string) {
	if b.db == nil {
		b.pendingAction = ""
		b.startNewLockFlow()
		return
	}

	cages, err := b.db.GetCages()
	if err != nil || len(cages) == 0 {
		b.pendingAction = ""
		b.startNewLockFlow()
		return
	}

	var num int
	fmt.Sscanf(strings.TrimSpace(text), "%d", &num)
	if num < 1 || num > len(cages) {
		lines := []string{"❌ Número inválido. ¿Cuál jaula tienes puesta?"}
		for i, c := range cages {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, c.Name))
		}
		b.Send(strings.Join(lines, "\n"))
		return
	}

	selected := cages[num-1]
	b.db.SetToyInUse(selected.ID, true)
	b.reloadToysFromDB()
	b.pendingAction = ""
	b.mustSaveState()

	b.Send(fmt.Sprintf("_Jaula seleccionada: *%s*_", selected.Name))
	b.startNewLockFlow()
}

// HandleExplain explica cómo completar y fotografiar la tarea actual
func (b *Bot) HandleExplain() {
	if b.state.CurrentTask == nil || b.state.CurrentTask.Completed || b.state.CurrentTask.Failed {
		b.Send("No hay tarea activa en este momento.")
		return
	}

	b.Send("_Analizando la tarea..._")

	explanation, err := b.ai.GenerateTaskExplanation(
		b.state.CurrentTask.Description,
		b.state.Toys,
		b.daysLocked(),
	)
	if err != nil {
		b.Send("❌ Error generando explicación.")
		return
	}

	b.Send(fmt.Sprintf(
		"▪️ *CÓMO COMPLETAR LA TAREA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_",
		b.state.CurrentTask.Description,
		stripMarkdown(explanation),
	))
}

// HandleToyPhoto procesa la foto de un juguete nuevo, llama a la IA para nombre/descripción
// y lo guarda en DB + Cloudinary
func (b *Bot) HandleToyPhoto(imageBytes []byte, mimeType string) {
	b.pendingAction = ""

	b.Send("_Analizando el juguete..._")

	// IA genera nombre y descripción
	toyInfo, err := b.ai.DescribeToy(imageBytes, mimeType, "")
	if err != nil || toyInfo == nil {
		b.Send("❌ Error analizando la foto del juguete.")
		return
	}

	// Subir foto a Cloudinary
	var photoURL, photoPublicID string
	if b.cloudinary != nil {
		url, pid, err := b.cloudinary.Upload(imageBytes, mimeType, "chaster/toys")
		if err != nil {
			log.Printf("error subiendo foto de juguete: %v", err)
		} else {
			photoURL = url
			photoPublicID = pid
		}
	}

	// Generar ID único
	toyID := fmt.Sprintf("toy-%d", time.Now().UnixNano())

	// Guardar en DB
	if b.db != nil {
		b.db.SaveToy(&storage.Toy{
			ID: toyID, Name: toyInfo.Name,
			Description:   toyInfo.Description,
			PhotoURL:      photoURL,
			PhotoPublicID: photoPublicID,
			Type:          toyInfo.Type,
			CreatedAt:     time.Now(),
		})
	}

	// Añadir al estado en memoria
	b.state.Toys = append(b.state.Toys, models.Toy{
		ID: toyID, Name: toyInfo.Name,
		Description:   toyInfo.Description,
		PhotoURL:      photoURL,
		PhotoPublicID: photoPublicID,
		Type:          toyInfo.Type,
		AddedAt:       time.Now(),
	})
	b.mustSaveState()

	b.Send(fmt.Sprintf(
		"✅ *%s* añadido al inventario.\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_",
		toyInfo.Name, toyInfo.Description,
	))
}

// ── Chat libre ────────────────────────────────────────────────────────────

func (b *Bot) HandleChat(text string) {
	// Rate limiting — máximo 1 mensaje IA cada 3 segundos
	b.chatMu.Lock()
	if time.Since(b.lastChatTime) < 3*time.Second {
		b.chatMu.Unlock()
		return
	}
	b.lastChatTime = time.Now()
	b.chatMu.Unlock()

	// Verificar expiración de edge pendiente
	if b.state.EdgePendingAt != nil && time.Now().After(b.state.EdgePendingAt.Add(2*time.Hour)) {
		b.state.EdgePendingAt = nil
		b.state.EdgeCount = 0
		b.state.TasksStreak = max(0, b.state.TasksStreak-1)
		b.pendingAction = ""
		b.mustSaveState()
		b.addWeeklyDebt("edge no confirmado a tiempo")
		b.Send("▪️ *TIEMPO AGOTADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_No confirmaste el edge. -1 obediencia. Anotado._")
		return
	}

	textLower := strings.ToLower(text)

	// Ritual matutino — respuesta de texto
	if b.pendingAction == "ritual_message" {
		b.HandleRitualMessage(text)
		return
	}

	// Detectar selección de jaula durante flujo de newlock
	if b.pendingAction == "selecting_cage" {
		b.handleCageSelection(text)
		return
	}

	// Detectar selección de juguete a eliminar
	if b.pendingAction == "removing_toy" {
		b.handleToyRemoveSelection(text)
		return
	}

	// Confirmación de edge pendiente
	if b.pendingAction == "edge_confirm" {
		b.handleEdgeConfirmation(text)
		return
	}

	// Detectar selección de prenda a eliminar
	if b.pendingAction == "removing_clothing" {
		b.handleClothingRemoveSelection(text)
		return
	}

	// Detectar ruegos sobre evento activo (freeze/hidetime)
	if b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt) {
		eventKeywords := map[string][]string{
			"freeze":   {"descongela", "unfreeze", "congela", "frio", "fría", "congelada", "liberame", "libérame"},
			"hidetime": {"timer", "tiempo", "cuanto", "cuánto", "falta", "muestrame", "muéstrame", "ver el tiempo"},
		}
		for _, kw := range eventKeywords[b.state.ActiveEvent.Type] {
			if strings.Contains(textLower, kw) {
				b.handleEventNegotiation(text)
				return
			}
		}
	}

	// Detectar ruego de permiso de orgasmo
	orgasmKeywords := []string{
		"masturbar", "correrme", "correrse", "orgasmo", "venirme", "acabar",
		"tocarme", "dildo", "permiso para", "puedo usar", "puedo meter",
		"puedo cogerme", "follarme", "usarme el culo",
	}
	for _, kw := range orgasmKeywords {
		if strings.Contains(textLower, kw) {
			b.handleOrgasmRequest(text)
			return
		}
	}

	// Detectar negociación de tiempo
	negotiationKeywords := []string{
		"quitar", "reducir", "menos tiempo", "recompensa", "me porté",
		"porte bien", "negociar", "tiempo", "horas", "minutos", "liberar",
		"permiso", "puedo", "déjame", "por favor",
	}

	isNegotiation := false
	for _, kw := range negotiationKeywords {
		if strings.Contains(textLower, kw) {
			isNegotiation = true
			break
		}
	}

	if isNegotiation {
		b.handleNegotiation(text)
		return
	}

	_, lockErr := b.chaster.GetActiveLock()
	locked := lockErr == nil

	var history []models.ChatMessage
	var rules []models.ContractRule
	if b.db != nil {
		history, _ = b.db.GetRecentChatHistory(6, 120)
		if locked {
			rules, _ = b.db.GetActiveContractRules()
		}
	}

	result, err := b.ai.Chat(
		text,
		b.state.Toys,
		b.daysLocked(),
		b.state.TasksCompleted,
		b.state.TasksFailed,
		b.state.TotalTimeAddedHours,
		locked,
		history,
		rules,
	)
	if err != nil {
		b.Send("_..._")
		return
	}

	if b.db != nil {
		b.db.SaveChatMessage("user", text)
		b.db.SaveChatMessage("assistant", result.Message)
	}

	b.Send(stripMarkdown(result.Message))

	if result.Violation != nil {
		b.handleContractViolation(result.Violation)
	}
}

// handleContractViolation ejecuta el castigo cuando la IA detecta una infracción al contrato.
func (b *Bot) handleContractViolation(v *ai.ChatViolation) {
	if b.db == nil {
		return
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}

	hours := v.Hours
	minutes := v.Minutes

	// Reincidencia en las últimas 24h → doble castigo
	prev := b.db.CountRecentViolations(v.RuleID, 24)
	if prev > 0 {
		hours *= 2
		minutes *= 2
	}

	b.db.LogViolation(v.RuleID, v.RuleText, v.Punishment, hours, minutes)

	reincidencia := prev > 0
	suffix := ""
	if reincidencia {
		suffix = "\n_Doble castigo por reincidencia._"
	}

	switch v.Punishment {
	case "add_time":
		if hours <= 0 {
			hours = 1
		}
		if err := b.chaster.AddTime(lock.ID, hours*3600); err != nil {
			log.Printf("[ContractViolation] error add_time: %v", err)
			return
		}
		b.state.TotalTimeAddedHours += hours
		b.mustSaveState()
		b.Send(fmt.Sprintf("⚠️ *INFRACCIÓN* — +%dh por: _%s_%s", hours, v.RuleText, suffix))

	case "pillory":
		if minutes <= 0 {
			minutes = 15
		}
		reason := fmt.Sprintf("Contract violation: %s", v.RuleText)
		engReason, rerr := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, reason)
		if rerr != nil || strings.TrimSpace(engReason) == "" {
			engReason = "Contract violation"
		}
		if err := b.chaster.PutInPillory(lock.ID, minutes*60, strings.TrimSpace(engReason)); err != nil {
			log.Printf("[ContractViolation] error pillory: %v", err)
			return
		}
		b.Send(fmt.Sprintf("⛓ *INFRACCIÓN* — %dmin en el cepo por: _%s_%s", minutes, v.RuleText, suffix))

	case "freeze":
		if minutes <= 0 {
			minutes = 30
		}
		if err := b.chaster.FreezeLock(lock.ID); err != nil {
			log.Printf("[ContractViolation] error freeze: %v", err)
			return
		}
		expiresAt := time.Now().Add(time.Duration(minutes) * time.Minute)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "freeze", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf("❄️ *INFRACCIÓN* — %dmin congelada por: _%s_%s", minutes, v.RuleText, suffix))
	}
}

// rollOrgasmOutcome decide el resultado usando la tabla de probabilidades.
// consecutiveDenials: cuántos "denied" seguidos sin edge ni granted.
func rollOrgasmOutcome(streak, daysSinceLastGrant, consecutiveDenials int) string {
	type probs struct{ denied, edge, granted int }
	var p probs

	// Umbrales alineados con títulos: <4 maricona, 4-8 sissy, 9-14 culo, 15-20 puta obediente, 21+ esclava
	switch {
	case streak < 4: // maricona desobediente
		p = probs{85, 15, 0}
	case streak <= 8 && daysSinceLastGrant < 5: // sissy sin entrenar, poco tiempo
		p = probs{60, 35, 5}
	case streak <= 8 && daysSinceLastGrant < 10:
		p = probs{35, 45, 20}
	case streak <= 8:
		p = probs{20, 50, 30}
	case streak <= 14 && daysSinceLastGrant < 5: // culo en formación
		p = probs{45, 45, 10}
	case streak <= 14 && daysSinceLastGrant < 10:
		p = probs{15, 45, 40}
	case streak <= 14:
		p = probs{5, 35, 60}
	case daysSinceLastGrant < 5: // puta obediente / esclava
		p = probs{35, 45, 20}
	case daysSinceLastGrant < 10:
		p = probs{10, 35, 55}
	default: // esclava perfecta, mucho tiempo sin
		p = probs{5, 25, 70}
	}

	// Boost por racha de rechazos consecutivos
	if consecutiveDenials >= 5 {
		boost := min(p.denied, 25)
		p.denied -= boost
		p.edge += boost - 5
		p.granted += 5
	} else if consecutiveDenials >= 3 {
		boost := min(p.denied, 15)
		p.denied -= boost
		p.edge += boost
	}

	roll := rand.Intn(100)
	if roll < p.denied {
		return "denied"
	} else if roll < p.denied+p.edge {
		return "edge"
	}
	return "granted"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// orgasmCooldownHours devuelve las horas de cooldown según el último resultado.
func orgasmCooldownHours(lastOutcome string) time.Duration {
	switch lastOutcome {
	case "granted":
		return 24 * time.Hour
	case "edge":
		return 4 * time.Hour
	default: // "denied"
		return 6 * time.Hour
	}
}

// countConsecutiveDenials cuenta cuántos "denied" seguidos hay desde el último edge o granted.
func countConsecutiveDenials(db interface {
	GetOrgasmHistory(int) ([]*storage.OrgasmEntry, error)
}) int {
	entries, err := db.GetOrgasmHistory(20)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.Outcome == "denied" {
			count++
		} else {
			break
		}
	}
	return count
}

func (b *Bot) handleOrgasmRequest(text string) {
	// Verificar si ya hay un edge pendiente
	if b.state.EdgePendingAt != nil {
		if time.Now().Before(b.state.EdgePendingAt.Add(2 * time.Hour)) {
			b.Send("▪️ *EDGE PENDIENTE*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Todavía tienes una orden pendiente. Complétala primero._")
			return
		}
		b.state.EdgePendingAt = nil
		b.state.EdgeCount = 0
	}

	// Verificar cooldown
	if b.state.LastOrgasmRequestAt != nil {
		cooldown := orgasmCooldownHours(b.state.LastOrgasmOutcome)
		elapsed := time.Since(*b.state.LastOrgasmRequestAt)
		if elapsed < cooldown {
			hoursLeft := (cooldown - elapsed).Hours()
			msg, err := b.ai.GenerateOrgasmCooldownMessage(b.state.LastOrgasmOutcome, hoursLeft)
			if err != nil || strings.TrimSpace(msg) == "" {
				msg = fmt.Sprintf("Todavía no. Faltan %.0f horas.", hoursLeft)
			}
			b.Send("▪️ *DEMASIADO PRONTO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(msg))
			return
		}
	}

	b.Send("_..._")

	// Obtener historial
	daysSinceLastGrant := 999
	consecutiveDenials := 0
	if b.db != nil {
		if all, err := b.db.GetOrgasmHistory(50); err == nil {
			for _, e := range all {
				if e.Granted {
					daysSinceLastGrant = int(time.Since(e.CreatedAt).Hours()) / 24
					break
				}
			}
		}
		consecutiveDenials = countConsecutiveDenials(b.db)
	}

	// Tirar la ruleta
	outcome := rollOrgasmOutcome(b.state.TasksStreak, daysSinceLastGrant, consecutiveDenials)

	decision, err := b.ai.GenerateOrgasmMessage(
		outcome, text,
		b.state.Toys, b.daysLocked(),
		b.state.TasksStreak, daysSinceLastGrant, consecutiveDenials,
	)
	if err != nil {
		b.Send("_..._")
		return
	}

	// Registrar cooldown
	now := time.Now()
	b.state.LastOrgasmRequestAt = &now
	b.state.LastOrgasmOutcome = outcome

	// Guardar en historial
	if b.db != nil {
		b.db.SaveOrgasmEntry(&storage.OrgasmEntry{
			Outcome:       outcome,
			UserMessage:   text,
			SenorResponse: decision.Message,
			Condition:     decision.Condition,
			StreakAtTime:  b.state.TasksStreak,
			DaysLocked:    b.daysLocked(),
		})
	}
	b.mustSaveState()

	switch outcome {
	case "granted":
		msg := "▪️ *PERMISO CONCEDIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(decision.Message)
		if strings.TrimSpace(decision.Condition) != "" {
			msg += "\n▬▬▬▬▬▬▬▬▬▬▬▬\n_" + stripMarkdown(decision.Condition) + "_"
		}
		b.Send(msg)

	case "edge":
		b.state.EdgePendingAt = &now
		b.pendingAction = "edge_confirm"
		b.mustSaveState()

		msg := "▪️ *ORDEN DE EDGE*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(decision.Message)
		if strings.TrimSpace(decision.Condition) != "" {
			msg += "\n▬▬▬▬▬▬▬▬▬▬▬▬\n_" + stripMarkdown(decision.Condition) + "_"
		}
		msg += "\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Confirma cuando hayas terminado. Tienes 2 horas._"
		b.Send(msg)

	default: // denied
		b.Send("▪️ *DENEGADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(decision.Message))
	}
}

// handleEdgeConfirmation procesa la confirmación de un edge completado
func (b *Bot) handleEdgeConfirmation(text string) {
	if b.state.EdgePendingAt == nil {
		b.pendingAction = ""
		return
	}

	// Verificar timeout
	if time.Now().After(b.state.EdgePendingAt.Add(2 * time.Hour)) {
		b.state.EdgePendingAt = nil
		b.state.EdgeCount = 0
		b.state.TasksStreak = max(0, b.state.TasksStreak-1)
		b.pendingAction = ""
		b.mustSaveState()
		b.addWeeklyDebt("edge no confirmado a tiempo")
		b.Send("▪️ *TIEMPO AGOTADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_No confirmaste el edge a tiempo. -1 obediencia. Anotado._")
		return
	}

	confirmKeywords := []string{"listo", "hecho", "confirmado", "hice", "cumplido", "terminé", "termine", "completo", "completé"}
	textLower := strings.ToLower(text)
	confirmed := false
	for _, kw := range confirmKeywords {
		if strings.Contains(textLower, kw) {
			confirmed = true
			break
		}
	}

	if !confirmed {
		b.Send("_¿Ya lo hiciste? Confirma cuando termines el edge._")
		return
	}

	b.state.EdgePendingAt = nil
	b.state.EdgeCount = 0
	b.pendingAction = ""
	b.mustSaveState()

	b.Send("▪️ *EDGE CONFIRMADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Al borde y sin correrte. Como debe ser._")
}

func (b *Bot) HandleOrgasmHistory() {
	if b.db == nil {
		b.Send("❌ Base de datos no disponible.")
		return
	}

	total, granted, edged, denied, err := b.db.GetOrgasmStats()
	if err != nil || total == 0 {
		b.Send("💦 *HISTORIAL DE ORGASMOS*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Ningún registro todavía._")
		return
	}

	entries, err := b.db.GetOrgasmHistory(10)
	if err != nil {
		b.Send("❌ Error obteniendo historial.")
		return
	}

	loc, _ := time.LoadLocation("America/Bogota")
	lines := []string{fmt.Sprintf(
		"💦 *HISTORIAL DE ORGASMOS*\n▬▬▬▬▬▬▬▬▬▬▬▬\n✅ Concedidos — *%d*\n🌊 Edges — *%d*\n❌ Denegados — *%d*\nTotal — *%d*\n▬▬▬▬▬▬▬▬▬▬▬▬",
		granted, edged, denied, total,
	)}

	for _, e := range entries {
		icon := "❌"
		status := "DENEGADO"
		switch e.Outcome {
		case "granted":
			icon = "✅"
			status = "CONCEDIDO"
		case "edge":
			icon = "🌊"
			status = "EDGE"
		}
		date := e.CreatedAt.In(loc).Format("02 Jan 15:04")
		lines = append(lines, fmt.Sprintf(
			"%s *%s* — %s\n_Racha: %d | Día %d_\n_%s_",
			icon, status, date,
			e.StreakAtTime, e.DaysLocked,
			stripMarkdown(e.SenorResponse),
		))
	}

	b.Send(strings.Join(lines, "\n▬▬▬▬▬▬▬▬▬▬▬▬\n"))
}

func (b *Bot) handleNegotiation(text string) {
	b.Send("_Evaluando tu petición..._")

	result, err := b.ai.NegotiateTime(
		text,
		b.state.Toys,
		b.daysLocked(),
		b.state.TasksCompleted,
		b.state.TasksFailed,
		b.state.TotalTimeAddedHours,
	)
	if err != nil {
		b.Send("_..._")
		return
	}

	lock, _ := b.chaster.GetActiveLock()

	switch result.Decision {
	case "approved":
		if result.TimeHours < 0 && lock != nil {
			hoursToRemove := -result.TimeHours
			if err := b.chaster.RemoveTime(lock.ID, hoursToRemove*3600); err != nil {
				log.Printf("error quitando tiempo en negociación: %v", err)
			}
			b.state.TotalTimeRemovedHours += hoursToRemove
			b.mustSaveState()
		}
		timeStr := ""
		if result.TimeHours < 0 {
			timeStr = fmt.Sprintf("\n▬▬▬▬▬▬▬▬▬▬▬▬\n*%dh* quitadas de tu condena.", -result.TimeHours)
		}
		b.Send("▪️ *APROBADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(result.Message) + timeStr)

	case "rejected":
		b.Send("▪️ *RECHAZADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(result.Message))

	case "counter":
		b.Send(fmt.Sprintf(
			"▪️ *CONTRAOFERTA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Tarea: %s_",
			stripMarkdown(result.Message),
			stripMarkdown(result.CounterTask),
		))

	case "penalty":
		if lock != nil {
			if err := b.chaster.AddTime(lock.ID, result.TimeHours*3600); err != nil {
				log.Printf("error añadiendo tiempo como penalización: %v", err)
			}
			b.state.TotalTimeAddedHours += result.TimeHours
			b.mustSaveState()
		}
		b.Send(fmt.Sprintf(
			"▪️ *PENALIZACIÓN*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas.",
			stripMarkdown(result.Message),
			result.TimeHours,
		))
	}
}

// ── Inventario ─────────────────────────────────────────────────────────────

func (b *Bot) HandleToys(args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	subCmd := ""
	if len(parts) > 0 {
		subCmd = strings.ToLower(parts[0])
	}
	_ = parts // toyName ya no se usa — la IA genera todo desde la foto

	switch subCmd {
	case "add", "agregar":
		b.pendingAction = "new_toy"
		b.Send("▪️ *NUEVO JUGUETE*\n▬▬▬▬▬▬▬▬▬▬▬▬\nManda la foto del juguete.\n_La IA generará nombre, descripción y tipo automáticamente._")

	case "remove", "quitar":
		// Mostrar lista para seleccionar
		if len(b.state.Toys) == 0 {
			b.Send("No hay juguetes en el inventario.")
			return
		}
		lines := []string{"▪️ *¿CUÁL QUIERES ELIMINAR?*\n▬▬▬▬▬▬▬▬▬▬▬▬"}
		for i, t := range b.state.Toys {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, t.Name))
		}
		lines = append(lines, "▬▬▬▬▬▬▬▬▬▬▬▬\n_Responde con el número._")
		b.Send(strings.Join(lines, "\n"))
		b.pendingAction = "removing_toy"

	default:
		if len(b.state.Toys) == 0 {
			b.Send("🧸 *INVENTARIO*\n\nVacío. Añade juguetes con:\n`/toys add`")
			return
		}
		lines := []string{"🧸 *INVENTARIO*\n"}
		for i, t := range b.state.Toys {
			status := ""
			if t.InUse {
				status = " ✅"
			}
			typeStr := ""
			switch t.Type {
			case "cage":
				typeStr = " 🔒"
			case "plug":
				typeStr = " 🔌"
			case "dildo":
				typeStr = " 🍆"
			case "vibrator":
				typeStr = " 📳"
			case "nipple":
				typeStr = " 🌀"
			case "restraint":
				typeStr = " ⛓"
			}
			lines = append(lines, fmt.Sprintf("%d. %s%s%s", i+1, t.Name, typeStr, status))
		}
		lines = append(lines, "\n_✅ = en uso ahora_")
		lines = append(lines, "`/toys add` — añadir")
		lines = append(lines, "`/toys remove` — eliminar")
		b.Send(strings.Join(lines, "\n"))
	}
}

// ── History ────────────────────────────────────────────────────────────────

func (b *Bot) HandleHistory() {
	if b.db == nil {
		b.Send("❌ Base de datos no disponible.")
		return
	}
	tasks, err := b.db.GetRecentTasks(10)
	if err != nil || len(tasks) == 0 {
		b.Send("📋 *HISTORIAL*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Sin tareas registradas todavía._")
		return
	}
	loc, _ := time.LoadLocation("America/Bogota")
	lines := []string{"📋 *HISTORIAL — últimas tareas*\n▬▬▬▬▬▬▬▬▬▬▬▬"}
	for _, t := range tasks {
		icon := "⏳"
		switch t.Status {
		case "completed":
			icon = "✅"
		case "failed":
			icon = "💀"
		}
		date := t.AssignedAt.In(loc).Format("02 Jan")
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s %s\n_%s_", icon, date, desc))
	}
	b.Send(strings.Join(lines, "\n▬▬▬▬▬▬▬▬▬▬▬▬\n"))
}

// ── Mood ───────────────────────────────────────────────────────────────────

func (b *Bot) HandleMood() {
	if _, err := b.chaster.GetActiveLock(); err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	b.Send("_..._")
	msg, err := b.ai.GenerateMoodMessage(
		b.daysLocked(),
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		b.state.TasksStreak,
		b.state.WeeklyDebt,
	)
	if err != nil {
		b.Send("_..._")
		return
	}
	b.Send(stripMarkdown(msg))
}

// ── Stats ─────────────────────────────────────────────────────────────────────

// HandleStats muestra estadísticas históricas desde la DB
func (b *Bot) HandleStats() {
	if b.db == nil {
		b.Send("❌ Base de datos no disponible.")
		return
	}
	stats, err := b.db.GetStats()
	if err != nil {
		b.Send("❌ Error obteniendo estadísticas.")
		return
	}

	taskTotal := stats.TotalTasksCompleted + stats.TotalTasksFailed
	rateStr := "—"
	if taskTotal > 0 {
		rate := (stats.TotalTasksCompleted * 100) / taskTotal
		rateStr = fmt.Sprintf("%d%%", rate)
	}

	b.Send(fmt.Sprintf(
		"▪️ *ESTADÍSTICAS HISTÓRICAS*\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"🔒 Sesiones — *%d*\n"+
			"📋 Tareas completadas — *%d*\n"+
			"💀 Tareas fallidas — *%d*\n"+
			"📊 Tasa de éxito — *%s*\n"+
			"⚡ Eventos — *%d*\n"+
			"🧸 Juguetes — *%d*\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"⏱ Tiempo añadido — *+%dh*\n"+
			"✅ Tiempo quitado — *-%dh*",
		stats.TotalLocks,
		stats.TotalTasksCompleted,
		stats.TotalTasksFailed,
		rateStr,
		stats.TotalEvents,
		stats.TotalToys,
		stats.TotalTimeAddedHours,
		stats.TotalTimeRemovedHours,
	))
}

// ── Help ───────────────────────────────────────────────────────────────────

func (b *Bot) HandleHelp() {
	b.Send(`🔒 *CHASTER KEYHOLDER BOT*
▬▬▬▬▬▬▬▬▬▬▬▬

📋 *SESIÓN Y TAREAS*
/status — Estado de tu condena
/task — Ver tarea activa del día
/explain — Cómo fotografiar la tarea
/fail — Confesar que fallaste
/roulette — Girar la ruleta diaria 🎰
/chatask — Tarea comunitaria de Chaster
/newlock — Iniciar nueva sesión
/contrato — Ver el contrato de sesión actual

🧸 *INVENTARIO*
/toys — Ver tus juguetes
/toys add — Añadir juguete
/toys remove — Eliminar juguete

👗 *GUARDARROPA*
/wardrobe — Ver prendas
/wardrobe add — Añadir prenda (foto)
/wardrobe remove — Eliminar prenda

📊 *HISTORIAL*
/stats — Estadísticas
/history — Últimas 10 tareas
/orgasms — Historial de permisos
/mood — Estado de ánimo de Papi

▬▬▬▬▬▬▬▬▬▬▬▬
_Para completar una tarea — manda la foto directo al chat._`)
}

// ── Loop principal ─────────────────────────────────────────────────────────

// syncLockState consulta Chaster al arrancar y persiste el estado del lock
// en state.json inmediatamente. Esto evita que el dashboard muestre "libre"
// tras un reinicio si el estado todavía no fue actualizado por los jobs.
func (b *Bot) syncLockState() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		log.Printf("[syncLockState] sin lock activo o error de API: %v", err)
		return
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24
	b.stateMu.Lock()
	b.state.DaysLocked = days
	b.cachedDaysLocked = days
	b.cachedDaysLockedAt = time.Now()
	if b.state.CurrentLockID == "" {
		b.state.CurrentLockID = lock.ID
	}
	startUTC := lock.StartDate.UTC()
	b.state.LockStartDate = &startUTC
	if lock.EndDate != nil {
		endUTC := lock.EndDate.UTC()
		b.state.LockEndDate = &endUTC
	}
	b.stateMu.Unlock()
	b.mustSaveState()
	log.Printf("[syncLockState] lock activo: %s — %d días", lock.ID, days)
}

func (b *Bot) Start() {
	go b.syncLockState()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	// Keyboard sin duplicados, organizado
	keyboard := [][]tgbotapi.KeyboardButton{
		{tgbotapi.NewKeyboardButton("/status"), tgbotapi.NewKeyboardButton("/task")},
		{tgbotapi.NewKeyboardButton("/fail"), tgbotapi.NewKeyboardButton("/explain")},
		{tgbotapi.NewKeyboardButton("/roulette"), tgbotapi.NewKeyboardButton("/chatask")},
		{tgbotapi.NewKeyboardButton("/toys"), tgbotapi.NewKeyboardButton("/wardrobe")},
		{tgbotapi.NewKeyboardButton("/orgasms"), tgbotapi.NewKeyboardButton("/history")},
		{tgbotapi.NewKeyboardButton("/mood"), tgbotapi.NewKeyboardButton("/contrato")},
		{tgbotapi.NewKeyboardButton("/stats")},
		{tgbotapi.NewKeyboardButton("/help")},
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != b.chatID {
			continue
		}

		// ── Foto recibida ──────────────────────────────────────────────
		if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
			largest := update.Message.Photo[len(update.Message.Photo)-1]
			imgBytes, mime, err := b.downloadFile(largest.FileID)
			if err != nil {
				b.Send("❌ Error descargando la foto.")
				continue
			}

			msgID := update.Message.MessageID

			if b.state.AwaitingLockPhoto {
				b.HandleLockPhoto(imgBytes, mime, msgID)
			} else if b.pendingAction == "new_toy" {
				b.deleteMessage(msgID)
				b.HandleToyPhoto(imgBytes, mime)
			} else if b.pendingAction == "ritual_photo" {
				b.deleteMessage(msgID)
				b.HandleRitualPhoto(imgBytes, mime)
			} else if b.pendingAction == "plug_photo" {
				b.deleteMessage(msgID)
				b.HandlePlugPhoto(imgBytes, mime)
			} else if b.pendingAction == "checkin_photo" ||
				(b.state.PendingCheckin && b.state.CheckinExpiresAt != nil && time.Now().Before(*b.state.CheckinExpiresAt)) {
				b.pendingAction = "checkin_photo"
				b.deleteMessage(msgID)
				b.HandleCheckinPhoto(imgBytes, mime)
			} else if b.pendingAction == "chaster_task_photo" {
				b.deleteMessage(msgID)
				b.HandleChasterTaskPhoto(imgBytes, mime)
			} else if b.pendingAction == "new_clothing" {
				b.deleteMessage(msgID)
				b.HandleWardrobePhoto(imgBytes, mime)
			} else if b.pendingAction == "outfit_photo" {
				b.deleteMessage(msgID)
				b.HandleOutfitPhoto(imgBytes, mime)
			} else {
				b.deleteMessage(msgID)
				b.HandlePhoto(imgBytes, mime)
			}
			continue
		}

		// ── Comandos de texto ──────────────────────────────────────────
		text := update.Message.Text
		switch {
		case text == "/start":
			b.SendWithKeyboard("▪️ *KEYHOLDER ACTIVO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Estás bajo control._", keyboard)
		case text == "/status":
			b.HandleStatus()
		case text == "/task":
			b.HandleTask()
		case text == "/order":
			b.HandleTaskWithLevel("")
		case strings.HasPrefix(text, "/order "):
			b.HandleTaskWithLevel(strings.TrimPrefix(text, "/order "))
		case text == "/fail":
			b.HandleFail()
		case text == "/explain":
			b.HandleExplain()
		case text == "/newlock":
			b.HandleNewLock("")
		case strings.HasPrefix(text, "/newlock "):
			b.HandleNewLock(strings.TrimPrefix(text, "/newlock "))
		case text == "/help":
			b.HandleHelp()
		case text == "/stats":
			b.HandleStats()
		case text == "/orgasms":
			b.HandleOrgasmHistory()
		case text == "/history":
			b.HandleHistory()
		case text == "/mood":
			b.HandleMood()
		case text == "/toys":
			b.HandleToys("")
		case strings.HasPrefix(text, "/toys "):
			b.HandleToys(strings.TrimPrefix(text, "/toys "))
		case text == "/roulette":
			b.HandleRuleta()
		case text == "/chatask":
			b.HandleChasterTaskCommand()
		case text == "/wardrobe":
			b.HandleWardrobe("")
		case strings.HasPrefix(text, "/wardrobe "):
			b.HandleWardrobe(strings.TrimPrefix(text, "/wardrobe "))
		case text == "/contrato":
			b.HandleContrato()
		case text == "/quitar":
			b.HandleRemoveTime("")
		case strings.HasPrefix(text, "/quitar "):
			b.HandleRemoveTime(strings.TrimPrefix(text, "/quitar "))
		case text == "/dbwipe":
			b.HandleDBWipe()
		case text != "" && !strings.HasPrefix(text, "/"):
			b.HandleChat(text)
		}
	}
}

// parsePilloryCommand parsea "/pillory 30 razón del cepo"
func (b *Bot) parsePilloryCommand(args string) {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		b.Send("Uso: `/pillory [minutos] [razón opcional]`")
		return
	}
	var minutes int
	fmt.Sscanf(parts[0], "%d", &minutes)
	if minutes <= 0 {
		b.Send("❌ Los minutos deben ser un número positivo.")
		return
	}
	reason := "El amo lo ha decidido."
	if len(parts) > 1 {
		reason = strings.Join(parts[1:], " ")
	}
	b.HandlePillory(minutes, reason)
}

// ── Scheduler hooks ────────────────────────────────────────────────────────

func (b *Bot) SendMorningStatus() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24

	var timeRemaining string
	if lock.EndDate != nil {
		timeRemaining = chaster.FormatDuration(int64(time.Until(*lock.EndDate).Seconds()))
	} else {
		timeRemaining = "indefinido"
	}

	msg, _ := b.ai.GenerateMorningMessage(days, timeRemaining, b.state.Toys)
	b.Send("🌅 *BUENOS DÍAS*\n\n" + msg)
}

func (b *Bot) SendNightStatus() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24
	taskCompleted := b.state.CurrentTask != nil && b.state.CurrentTask.Completed

	// Penalizar tarea no completada al final del día
	if b.state.CurrentTask != nil && !b.state.CurrentTask.Completed && !b.state.CurrentTask.Failed {
		penaltyHours := b.state.CurrentTask.PenaltyHours
		if err := b.chaster.AddTime(lock.ID, penaltyHours*3600); err != nil {
			log.Printf("error añadiendo penalización nocturna: %v", err)
		}
		b.state.CurrentTask.Failed = true
		b.state.TotalTimeAddedHours += penaltyHours
		b.state.TasksFailed++
		b.state.TasksStreak = max(0, b.state.TasksStreak-3)
		b.addWeeklyDebt("tarea del día no completada")
		// Guardar tarea fallida en DB
		if b.db != nil {
			b.db.SaveTask(&storage.Task{
				ID: b.state.CurrentTask.ID, LockID: b.state.CurrentLockID,
				Description: b.state.CurrentTask.Description,
				AssignedAt:  b.state.CurrentTask.AssignedAt, DueAt: b.state.CurrentTask.DueAt,
				Status:       "failed",
				PenaltyHours: penaltyHours, RewardHours: b.state.CurrentTask.RewardHours,
			})
		}
	}

	msg, _ := b.ai.GenerateNightMessage(days, taskCompleted, b.state.Toys)
	b.Send("🌙 *BUENAS NOCHES*\n\n" + msg)

	b.state.CurrentTask = nil
	b.mustSaveState()
}

// ── Nuevo lock ─────────────────────────────────────────────────────────────

// parseDuration parsea strings en español e inglés a segundos
// Soporta: "4 horas", "1 hora", "2 días", "1 dia", "30 minutos", "1 semana"
// y en inglés: "4 hours", "1 day", "2 weeks"
func parseDuration(input string) int {
	input = strings.ToLower(strings.TrimSpace(input))
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return 0
	}

	var amount int
	fmt.Sscanf(parts[0], "%d", &amount)
	if amount <= 0 {
		return 0
	}

	unit := parts[1]
	switch {
	case strings.HasPrefix(unit, "minuto") || strings.HasPrefix(unit, "min"):
		return amount * 60
	case strings.HasPrefix(unit, "hora") || strings.HasPrefix(unit, "hour") || strings.HasPrefix(unit, "hr"):
		return amount * 3600
	case strings.HasPrefix(unit, "día") || strings.HasPrefix(unit, "dia") || strings.HasPrefix(unit, "day"):
		return amount * 86400
	case strings.HasPrefix(unit, "semana") || strings.HasPrefix(unit, "week"):
		return amount * 604800
	}
	return 0
}

func (b *Bot) HandleNewLock(args string) {
	if _, err := b.chaster.GetActiveLock(); err == nil {
		b.Send("🔒 Ya tienes una sesión activa. Espera a que termine antes de crear una nueva.")
		return
	}

	if args != "" {
		secs := parseDuration(args)
		if secs <= 0 {
			b.Send("❌ Formato inválido. Ejemplos:\n`/newlock 4 horas`\n`/newlock 1 hora`\n`/newlock 2 días`\n`/newlock 30 minutos`\n`/newlock 1 semana`")
			return
		}
		b.state.ManualDurationSeconds = secs
	} else {
		b.state.ManualDurationSeconds = 0
	}

	// Si hay jaulas registradas, preguntar cuál tiene puesta
	if b.db != nil {
		cages, err := b.db.GetCages()
		if err == nil && len(cages) > 0 {
			if len(cages) == 1 {
				// Solo una jaula — marcarla automáticamente
				b.db.SetToyInUse(cages[0].ID, true)
				b.reloadToysFromDB()
				b.Send(fmt.Sprintf("_Jaula registrada: *%s*_", cages[0].Name))
			} else {
				// Varias jaulas — preguntar cuál tiene puesta
				lines := []string{"▪️ *¿CUÁL JAULA TIENES PUESTA?*\n▬▬▬▬▬▬▬▬▬▬▬▬"}
				for i, c := range cages {
					lines = append(lines, fmt.Sprintf("%d. %s", i+1, c.Name))
				}
				lines = append(lines, "▬▬▬▬▬▬▬▬▬▬▬▬\n_Responde con el número._")
				b.Send(strings.Join(lines, "\n"))
				b.pendingAction = "selecting_cage"
				b.mustSaveState() // persiste ManualDurationSeconds
				return
			}
		}
	}

	b.startNewLockFlow()
}

func storageToyToModel(t *storage.Toy) models.Toy {
	return models.Toy{
		ID: t.ID, Name: t.Name, Description: t.Description,
		PhotoURL: t.PhotoURL, Type: t.Type, InUse: t.InUse,
		AddedAt: t.CreatedAt,
	}
}

// reloadToysFromDB recarga los juguetes desde la DB al estado en memoria
func (b *Bot) reloadToysFromDB() {
	if b.db == nil {
		return
	}
	toys, err := b.db.GetToys()
	if err != nil {
		return
	}
	b.state.Toys = make([]models.Toy, 0, len(toys))
	for _, t := range toys {
		b.state.Toys = append(b.state.Toys, storageToyToModel(t))
	}
}

func (b *Bot) startNewLockFlow() {
	b.Send("▪️ *NUEVA SESIÓN*\n▬▬▬▬▬▬▬▬▬▬▬▬\nCierra el candado. Gira los diales sin mirar.\n\nCuando esté listo, manda la foto.\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La imagen será eliminada automáticamente._")
	b.state.AwaitingLockPhoto = true
	b.mustSaveState()
}

func (b *Bot) HandleLockPhoto(imageBytes []byte, mimeType string, messageID int) {
	b.deleteMessage(messageID)
	b.Send("_Verificando..._")

	verdict, err := b.ai.VerifyLockPhoto(imageBytes, mimeType)
	if err != nil {
		b.Send("❌ Error analizando la foto. Inténtalo de nuevo.")
		return
	}

	if verdict.Status != "approved" {
		b.Send(fmt.Sprintf("❌ *Foto rechazada*\n\n_%s_\n\nAsegúrate de que el candado esté cerrado y la combinación sea visible en los diales.", verdict.Reason))
		return
	}

	b.Send("_Candado verificado. Creando sesión..._")

	var durationSeconds int
	var lockMsg string

	if b.state.ManualDurationSeconds > 0 {
		durationSeconds = b.state.ManualDurationSeconds
		hours := durationSeconds / 3600
		mins := (durationSeconds % 3600) / 60
		if hours > 0 {
			lockMsg = fmt.Sprintf("Tú lo pediste: %dh encerrada. Disfrútalo, esclava.", hours)
		} else {
			lockMsg = fmt.Sprintf("Tú lo pediste: %d minutos encerrada.", mins)
		}
		b.state.ManualDurationSeconds = 0
	} else {
		decision, err := b.ai.DecideLockDuration(b.daysLocked(), b.state.Toys)
		if err != nil {
			decision = &ai.LockDecision{DurationHours: 24, Message: "24 horas. Sin discusión."}
		}
		durationSeconds = decision.DurationHours * 3600
		lockMsg = decision.Message
	}

	combinationID, err := b.chaster.UploadCombinationImage(imageBytes, mimeType)
	if err != nil {
		b.Send("❌ Error subiendo la combinación a Chaster.")
		return
	}

	lockID, err := b.chaster.CreateLock(combinationID, durationSeconds)
	if err != nil {
		log.Printf("[CreateLock] error: %v", err)
		b.Send(fmt.Sprintf("❌ Error creando el lock en Chaster.\n`%v`", err))
		return
	}

	b.state.AwaitingLockPhoto = false
	b.state.CurrentLockID = lockID
	b.mustSaveState()

	// Guardar lock en DB
	if b.db != nil {
		b.db.SaveLock(&storage.Lock{
			ID:            fmt.Sprintf("lock-%s", lockID),
			ChasterID:     lockID,
			StartedAt:     time.Now(),
			DurationHours: durationSeconds / 3600,
		})
	}

	hours := durationSeconds / 3600
	mins := (durationSeconds % 3600) / 60
	var durStr string
	if hours > 0 {
		durStr = fmt.Sprintf("%dh", hours)
	} else {
		durStr = fmt.Sprintf("%dm", mins)
	}

	b.Send(fmt.Sprintf(
		"▪️ *SESIÓN INICIADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\nDuración — *%s*\n\n_Tu combinación está guardada. No la verás hasta que termine._",
		stripMarkdown(lockMsg),
		durStr,
	))

	// Generar y enviar contrato de sesión
	if b.db != nil {
		go func() {
			contractText, err := b.ai.GenerateContract(durationSeconds/3600, b.state.Toys, b.daysLocked())
			if err != nil {
				log.Printf("[GenerateContract] error: %v", err)
				return
			}
			b.db.SaveContract(&storage.Contract{
				ID:        fmt.Sprintf("contract-%d", time.Now().UnixNano()),
				LockID:    lockID,
				Text:      contractText,
				CreatedAt: time.Now(),
			})
			b.Send("📜 *CONTRATO DE SESIÓN*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(contractText))

			// Extraer reglas verificables por chat
			rules, rerr := b.ai.ExtractContractRules(contractText, lockID)
			if rerr != nil {
				log.Printf("[ExtractContractRules] error: %v", rerr)
			} else if len(rules) > 0 {
				if serr := b.db.SaveContractRules(rules); serr != nil {
					log.Printf("[SaveContractRules] error: %v", serr)
				} else {
					log.Printf("[Contract] %d reglas activas extraídas del contrato", len(rules))
				}
			}
		}()
	}
}

// finishLock ejecuta el cierre completo de una sesión: manda la combinación,
// archiva el lock en Chaster, actualiza la DB y limpia el estado.
// Llamado tanto desde CheckLockFinished como desde HandleStatus (auto-unlock).
func (b *Bot) finishLock(lockID string) {
	// Paso 1 — desbloquear el lock (requerido antes de poder leer la combinación)
	if err := b.chaster.UnlockLock(lockID); err != nil {
		log.Printf("[finishLock] error desbloqueando: %v", err)
		b.Send("❌ Error al desbloquear. Inténtalo desde la app de Chaster.")
		return
	}

	// Paso 2 — obtener la combinación (solo disponible después del unlock)
	combo, err := b.chaster.GetCombination(lockID)
	if err != nil {
		log.Printf("[finishLock] error obteniendo combinación: %v", err)
		b.Send("❌ No pude obtener la combinación. Revisa Chaster directamente.")
		return
	}

	imgBytes, err := b.chaster.DownloadCombinationImage(combo.ImageFullURL)
	if err != nil {
		log.Printf("[finishLock] error descargando imagen: %v", err)
		b.Send("🔓 *SESIÓN TERMINADA*\n\nNo pude obtener la imagen de combinación. Revisa Chaster directamente.")
	} else {
		photoMsg := tgbotapi.NewPhoto(b.chatID, tgbotapi.FileBytes{
			Name:  "combinacion.jpg",
			Bytes: imgBytes,
		})
		photoMsg.Caption = "▪️ *SESIÓN TERMINADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\nEsta es tu combinación.\nYa puedes liberarte."
		photoMsg.ParseMode = "Markdown"
		if _, err := b.api.Send(photoMsg); err != nil {
			log.Printf("[finishLock] error enviando foto: %v", err)
			b.Send("🔓 *SESIÓN TERMINADA* — revisa Chaster para ver tu combinación.")
		}
	}

	if err := b.chaster.ArchiveLock(lockID); err != nil {
		log.Printf("[finishLock] error archivando lock: %v", err)
	}

	if b.db != nil {
		b.db.UpdateLockEnd(
			fmt.Sprintf("lock-%s", lockID),
			time.Now(),
			b.state.TasksCompleted,
			b.state.TasksFailed,
			b.state.TotalTimeAddedHours,
			b.state.TotalTimeRemovedHours,
			0,
		)
	}

	if b.db != nil {
		b.db.ClearAllInUse()
		b.reloadToysFromDB()
		b.db.ClearContractRules()
		b.db.ClearChatHistory()
	}

	b.state.CurrentLockID = ""
	b.mustSaveState()
}

// CheckLockFinished verifica si el lock activo terminó.
// Usa GetActiveLock (lista) como comprobación primaria porque es el único endpoint
// que devuelve isReadyToUnlock. Solo recurre a GetLockByID cuando el lock ya
// no aparece en la lista (fue desbloqueado manualmente o archivado).
func (b *Bot) CheckLockFinished() {
	if b.state.CurrentLockID == "" {
		return
	}

	rawID := b.state.CurrentLockID
	alreadyNotified := strings.HasPrefix(rawID, "notified:")
	lockID := strings.TrimPrefix(rawID, "notified:")

	// Paso 1: consultar la lista activa — devuelve isReadyToUnlock correctamente
	activeLock, err := b.chaster.GetActiveLock()
	if err == nil && activeLock.ID == lockID {
		// Lock sigue activo — revisar si está listo para desbloquear
		if activeLock.IsReadyToUnlock {
			log.Printf("[CheckLockFinished] IsReadyToUnlock=true — ejecutando finishLock para %s", lockID)
			b.state.CurrentLockID = lockID
			b.finishLock(lockID)
			return
		}
		// Tiempo vencido pero usuario aún no presionó desbloquear
		if activeLock.EndDate != nil && time.Now().After(*activeLock.EndDate) {
			if !alreadyNotified {
				b.Send("🔓 *TIEMPO CUMPLIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\nAbre Chaster y presiona desbloquear.\nCuando lo confirmes, te mando la combinación.")
				b.state.CurrentLockID = "notified:" + lockID
				b.mustSaveState()
			}
		}
		return
	}

	// Paso 2: lock no está en la lista activa — verificar qué pasó
	lock, err := b.chaster.GetLockByID(lockID)
	if err != nil {
		if errors.Is(err, chaster.ErrLockNotFound) {
			log.Printf("[CheckLockFinished] lock %s devolvió 404 — ejecutando finishLock", lockID)
			b.state.CurrentLockID = lockID
			b.finishLock(lockID)
		} else {
			log.Printf("[CheckLockFinished] error consultando lock %s: %v", lockID, err)
		}
		return
	}

	if lock.Status == "unlocked" {
		log.Printf("[CheckLockFinished] status=unlocked — ejecutando finishLock para %s", lockID)
		b.state.CurrentLockID = lockID
		b.finishLock(lockID)
		return
	}

	log.Printf("[CheckLockFinished] status=%s — no proceder", lock.Status)
}

// ── Eventos random ─────────────────────────────────────────────────────────

// probabilidadEvento calcula la probabilidad de evento según hora y días encerrada.
// Horario activo: 8am-11pm. Fuera de ese rango siempre 0.
func probabilidadEvento(hour, daysLocked int) int {
	if hour < 8 || hour >= 23 {
		return 0
	}
	// Base por horario
	base := 0
	switch {
	case hour >= 18: // noche: 6pm-11pm
		base = 55
	case hour >= 12: // tarde: 12pm-6pm
		base = 35
	default: // mañana: 8am-12pm
		base = 15
	}
	// Bonus por días encerrada (+5% cada 3 días, máx +20%)
	bonus := (daysLocked / 3) * 5
	if bonus > 20 {
		bonus = 20
	}
	return base + bonus
}

// HandleRandomEvent evalúa si lanzar un evento random y lo ejecuta si procede.
// Llamado por el scheduler cada 30 minutos en horario activo.
func (b *Bot) HandleRandomEvent() {
	loc, _ := time.LoadLocation("America/Bogota")
	hour := time.Now().In(loc).Hour()

	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24

	prob := probabilidadEvento(hour, days)
	if prob == 0 {
		return
	}

	if rand.Intn(100) >= prob {
		return
	}

	hasActive := b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt)

	decision, err := b.ai.DecideRandomEvent(
		days,
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		hour,
		hasActive,
	)
	if err != nil || decision.Action == "none" {
		return
	}

	b.executeRandomEvent(lock.ID, decision)
}

// HandleRandomEventTest fuerza un evento random ignorando probabilidad y horario.
// Solo para testing — llamado con /testevent.
func (b *Bot) HandleRandomEventTest() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}

	loc, _ := time.LoadLocation("America/Bogota")
	hour := time.Now().In(loc).Hour()
	hasActive := b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt)

	b.Send("_Generando evento de prueba..._")

	decision, err := b.ai.DecideRandomEvent(
		b.daysLocked(),
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		hour,
		hasActive,
	)
	if err != nil {
		b.Send(fmt.Sprintf("❌ Error: %v", err))
		return
	}

	if decision.Action == "none" {
		b.Send("_La IA decidió no hacer nada este ciclo. Intenta de nuevo._")
		return
	}

	b.executeRandomEvent(lock.ID, decision)
}

// executeRandomEvent ejecuta la acción decidida por la IA y gestiona la auto-reversión
func (b *Bot) executeRandomEvent(lockID string, decision *ai.RandomEventDecision) {
	log.Printf("[RandomEvent] acción=%s duración=%dm razón=%s", decision.Action, decision.DurationMinutes, decision.Reason)

	switch decision.Action {

	case "chatask":
		// Si ya hay una tarea comunitaria activa, ignorar
		if b.state.PendingChasterTask != "" || b.state.ChasterTaskLockID != "" {
			log.Printf("[RandomEvent] chatask ignorado — tarea comunitaria ya activa")
			return
		}
		if decision.Message != "" {
			b.Send(stripMarkdown(decision.Message))
		}
		b.HandleChasterTaskCommand()

	case "freeze":
		if err := b.chaster.FreezeLock(lockID); err != nil {
			log.Printf("[RandomEvent] error freeze: %v", err)
			return
		}
		expiresAt := time.Now().Add(time.Duration(decision.DurationMinutes) * time.Minute)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "freeze", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf(
			"❄️ *CONGELADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Duración: %d minutos_",
			stripMarkdown(decision.Message), decision.DurationMinutes,
		))

	case "hidetime":
		if err := b.chaster.SetTimerVisibility(lockID, false); err != nil {
			log.Printf("[RandomEvent] error hidetime: %v", err)
			return
		}
		expiresAt := time.Now().Add(time.Duration(decision.DurationMinutes) * time.Minute)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "hidetime", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf(
			"🙈 *TIMER OCULTO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Duración: %d minutos_",
			stripMarkdown(decision.Message), decision.DurationMinutes,
		))

	case "pillory":
		pilloryReason, rerr := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, decision.Reason)
		if rerr != nil || strings.TrimSpace(pilloryReason) == "" {
			pilloryReason = "Sent to pillory by her keyholder"
		}
		pilloryReason = strings.TrimSpace(pilloryReason)
		if err := b.chaster.PutInPillory(lockID, decision.DurationMinutes*60, pilloryReason); err != nil {
			log.Printf("[RandomEvent] error pillory: %v", err)
			return
		}
		b.Send(fmt.Sprintf(
			"⛓ *AL CEPO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Duración: %d minutos_",
			stripMarkdown(decision.Message), decision.DurationMinutes,
		))

	case "addtime":
		hours := decision.DurationMinutes / 60
		if hours <= 0 {
			hours = 1
		}
		if err := b.chaster.AddTime(lockID, hours*3600); err != nil {
			log.Printf("[RandomEvent] error addtime: %v", err)
			return
		}
		b.state.TotalTimeAddedHours += hours
		b.mustSaveState()
		b.Send(fmt.Sprintf(
			"⏳ *TIEMPO AÑADIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas.",
			stripMarkdown(decision.Message), hours,
		))
	}
}

// ── Mensajes random ───────────────────────────────────────────────────────

// randomMessageTypes — se rota para forzar variedad en los mensajes espontáneos
var randomMessageTypes = []string{
	"possessive reminder — he's thinking of her caged, belonging to him",
	"small degrading order — something tiny to do alone right now, no photo (think about X, say something out loud, feel the cage)",
	"perverse comment — about her cage, her body, what Papi enjoys about owning her",
	"uncomfortable psychological question — about her submission, what she is, her secret desires",
	"veiled threat — something is coming, vague and unsettling, no details",
	"mocking observation — laugh quietly at her situation, her cage, her life as a sissy",
	"conditioning phrase — reinforce what she is and who she belongs to",
	"reference to her secret — about what he knows, what he could do with that information, his patience",
}

// sendRandomMessageInternal construye y envía el mensaje espontáneo del keyholder.
func (b *Bot) sendRandomMessageInternal() {
	_, lockErr := b.chaster.GetActiveLock()
	locked := lockErr == nil

	hasActive := b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt)
	activeType := ""
	if hasActive {
		activeType = b.state.ActiveEvent.Type
	}

	// Forzar variedad eligiendo un tipo aleatorio antes de llamar a la IA
	msgType := randomMessageTypes[rand.Intn(len(randomMessageTypes))]

	msg, err := b.ai.GenerateRandomMessage(
		b.daysLocked(),
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		hasActive,
		activeType,
		locked,
		msgType,
	)
	if err != nil {
		log.Printf("[SendRandomMessage] error: %v", err)
		return
	}

	b.Send(stripMarkdown(msg))
}

// SendRandomMessage manda un mensaje espontáneo del keyholder.
// Llamado por el scheduler en horario activo.
func (b *Bot) SendRandomMessage() {
	b.sendRandomMessageInternal()
}

// HandleRemoveTime quita N horas de la condena — /quitar [horas]
func (b *Bot) HandleRemoveTime(args string) {
	hours := 1
	if args != "" {
		fmt.Sscanf(strings.TrimSpace(args), "%d", &hours)
		if hours <= 0 {
			hours = 1
		}
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	if err := b.chaster.RemoveTime(lock.ID, hours*3600); err != nil {
		b.Send(fmt.Sprintf("❌ Error quitando tiempo: %v", err))
		return
	}
	b.state.TotalTimeRemovedHours += hours
	b.mustSaveState()
	b.Send(fmt.Sprintf("✂️ *-%dh* quitadas de tu condena.", hours))
}

// SendRandomMessageTest fuerza un mensaje random — solo para testing con /testmsg
func (b *Bot) SendRandomMessageTest() {
	b.chatMu.Lock()
	if time.Since(b.lastChatTime) < 5*time.Second {
		b.chatMu.Unlock()
		return
	}
	b.lastChatTime = time.Now()
	b.chatMu.Unlock()

	b.sendRandomMessageInternal()
}

// CheckActiveEventExpiry verifica si hay un evento activo que haya expirado y lo revierte.
// Llamado por el scheduler cada 5 minutos.
func (b *Bot) CheckActiveEventExpiry() {
	if b.state.ActiveEvent == nil {
		return
	}
	if time.Now().Before(b.state.ActiveEvent.ExpiresAt) {
		return
	}

	// El evento expiró — revertir
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		// Lock ya no activo — limpiar estado
		b.state.ActiveEvent = nil
		b.mustSaveState()
		return
	}

	eventType := b.state.ActiveEvent.Type
	b.state.ActiveEvent = nil
	b.mustSaveState()

	switch eventType {
	case "freeze":
		if err := b.chaster.UnfreezeLock(lock.ID); err != nil {
			log.Printf("[CheckExpiry] error unfreeze: %v", err)
			return
		}
		b.Send("🔥 *DESCONGELADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_El tiempo de congelación terminó. Por ahora._")

	case "hidetime":
		if err := b.chaster.SetTimerVisibility(lock.ID, true); err != nil {
			log.Printf("[CheckExpiry] error show time: %v", err)
			return
		}
		b.Send("👁 *TIMER RESTAURADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Ya puedes ver el tiempo de nuevo. Disfruta mientras dura._")
	}
}

// ── Helpers de nuevas funcionalidades ─────────────────────────────────────

// addWeeklyDebt registra una infracción en la deuda semanal
func (b *Bot) addWeeklyDebt(detail string) {
	b.state.WeeklyDebt++
	b.state.WeeklyDebtDetails = append(b.state.WeeklyDebtDetails, detail)
	b.mustSaveState()
}

func todayStr() string {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("COT", -5*60*60)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func (b *Bot) getAssignedPlugName() string {
	if b.state.AssignedPlugID == "" {
		return ""
	}
	for _, t := range b.state.Toys {
		if t.ID == b.state.AssignedPlugID {
			return t.Name
		}
	}
	return ""
}

func (b *Bot) autoPillory(reason string) {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}
	pilloryReason, perr := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, reason)
	if perr != nil || strings.TrimSpace(pilloryReason) == "" {
		pilloryReason = reason
	}
	pilloryReason = strings.TrimSpace(pilloryReason)
	if err := b.chaster.PutInPillory(lock.ID, 30*60, pilloryReason); err != nil {
		log.Printf("[autoPillory] error: %v", err)
		return
	}
	b.Send(fmt.Sprintf("⛓ *AL CEPO — 30 minutos*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_", reason))
}

func (b *Bot) checkStreakMilestone(prevTitle string, newPoints int) {
	newTitle := models.ObedienceTitle(newPoints)
	if newTitle == prevTitle {
		return
	}
	msg, err := b.ai.GenerateStreakReward(newPoints, b.daysLocked(), b.state.Toys)
	if err != nil || strings.TrimSpace(msg) == "" {
		return
	}
	b.Send(fmt.Sprintf("🏆 *NUEVO TÍTULO: %s*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s", strings.ToUpper(newTitle), stripMarkdown(msg)))
}

// ── Ritual matutino ────────────────────────────────────────────────────────

func (b *Bot) StartMorningRitual() {
	if b.state.LastRitualDate == todayStr() {
		return
	}
	if _, err := b.chaster.GetActiveLock(); err != nil {
		return
	}
	obedienceLevel := models.GetObedienceLevelFromPoints(b.state.TasksStreak)
	msg, err := b.ai.GenerateRitualIntro(b.daysLocked(), b.state.Toys, obedienceLevel)
	if err != nil {
		return
	}
	b.state.RitualStep = 1
	b.mustSaveState()
	b.pendingAction = "ritual_photo"
	b.Send("🌅 *RITUAL MATUTINO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(msg) + "\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Manda la foto de tu jaula puesta._")
}

func (b *Bot) HandleRitualPhoto(imgBytes []byte, mime string) {
	b.Send("_Verificando..._")
	verdict, err := b.ai.VerifyCheckinPhoto(imgBytes, mime, b.getAssignedPlugName())
	if err != nil {
		b.Send("❌ Error verificando la foto. Inténtalo de nuevo.")
		return
	}
	switch verdict.Status {
	case "approved":
		b.state.RitualStep = 2
		b.mustSaveState()
		b.pendingAction = "ritual_message"
		b.Send("✅ _Foto verificada._\n\nAhora escríbeme cómo empiezas el día. ¿Cómo te sientes con la jaula puesta?")
	case "retry", "rejected":
		b.Send(fmt.Sprintf("❌ *Foto rechazada*\n\n_%s_\n\nInténtalo de nuevo.", verdict.Reason))
	}
}

func (b *Bot) HandleRitualMessage(text string) {
	b.pendingAction = ""
	obedienceLevel := models.GetObedienceLevelFromPoints(b.state.TasksStreak)
	response, err := b.ai.GenerateRitualResponse(text, b.daysLocked(), b.state.Toys, obedienceLevel)
	if err != nil {
		response = "Bien. Tienes permiso para seguir con tu día. No olvides quién manda."
	}
	b.state.RitualStep = 0
	b.state.LastRitualDate = todayStr()
	b.mustSaveState()
	b.Send(stripMarkdown(response))
}

// ── Plug del día ───────────────────────────────────────────────────────────

func (b *Bot) SendPlugAssignment() {
	if b.state.AssignedPlugDate == todayStr() {
		return
	}
	if _, err := b.chaster.GetActiveLock(); err != nil {
		return
	}
	var plugs []models.Toy
	for _, t := range b.state.Toys {
		if t.Type == "plug" {
			plugs = append(plugs, t)
		}
	}
	if len(plugs) == 0 {
		return
	}
	selected := plugs[rand.Intn(len(plugs))]
	obedienceLevel := models.GetObedienceLevelFromPoints(b.state.TasksStreak)
	msg, err := b.ai.GeneratePlugAssignment(selected.Name, b.daysLocked(), obedienceLevel)
	if err != nil {
		return
	}
	b.state.AssignedPlugID = selected.ID
	b.state.AssignedPlugDate = todayStr()
	b.state.PlugConfirmed = false
	b.mustSaveState()
	b.pendingAction = "plug_photo"
	b.Send(fmt.Sprintf(
		"🔌 *PLUG DEL DÍA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n▬▬▬▬▬▬▬▬▬▬▬▬\nPlug asignado: *%s*\n\n_Cuando lo tengas puesto, manda la foto._",
		stripMarkdown(msg), selected.Name,
	))
}

func (b *Bot) HandlePlugPhoto(imgBytes []byte, mime string) {
	b.Send("_Verificando..._")
	plugName := b.getAssignedPlugName()
	if plugName == "" {
		b.pendingAction = ""
		return
	}
	verdict, err := b.ai.VerifyPlugPhoto(imgBytes, mime, plugName)
	if err != nil {
		b.Send("❌ Error verificando la foto. Inténtalo de nuevo.")
		return
	}
	switch verdict.Status {
	case "approved":
		b.pendingAction = ""
		b.state.PlugConfirmed = true
		b.state.PlugBonusAccum++
		plugMsg := fmt.Sprintf("✅ *PLUG CONFIRMADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s puesto, como debe ser. Que dure todo el día._", plugName)
		if b.state.PlugBonusAccum >= 2 {
			b.state.TasksStreak++
			b.state.PlugBonusAccum = 0
			plugMsg += "\n_+1 obediencia por constancia con el plug._"
		}
		b.mustSaveState()
		b.Send(plugMsg)
	case "retry":
		b.Send(fmt.Sprintf("⚠️ *Intenta de nuevo*\n\n_%s_", verdict.Reason))
	case "rejected":
		b.pendingAction = ""
		b.Send(fmt.Sprintf("❌ *Plug no detectado*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_", verdict.Reason))
		b.addWeeklyDebt(fmt.Sprintf("plug %s no confirmado", plugName))
		b.autoPillory(fmt.Sprintf("no llevó el %s asignado", plugName))
	}
}

// ── Check-ins espontáneos ──────────────────────────────────────────────────

func (b *Bot) TriggerCheckin() {
	if b.state.PendingCheckin {
		return
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}

	// Solicitar verificación a Chaster
	if err := b.chaster.RequestVerificationPicture(lock.ID); err != nil {
		log.Printf("[checkin] error solicitando verificación a Chaster: %v", err)
		return
	}

	// Obtener código de verificación actual
	code, err := b.chaster.GetVerificationPictureCode(lock.ID)
	if err != nil {
		log.Printf("[checkin] error obteniendo código de verificación: %v", err)
		return
	}

	plugName := b.getAssignedPlugName()
	msg, err := b.ai.GenerateCheckinRequest(b.daysLocked(), plugName)
	if err != nil {
		return
	}

	expiresAt := time.Now().Add(30 * time.Minute)
	b.state.PendingCheckin = true
	b.state.CheckinExpiresAt = &expiresAt
	b.state.CheckinReminderSent = false
	b.state.CheckinVerificationCode = code

	// Crear entrada en DB
	checkinID := fmt.Sprintf("checkin-%d", time.Now().UnixNano())
	b.state.CurrentCheckinID = checkinID
	if b.db != nil {
		b.db.SaveCheckin(&storage.CheckinEntry{
			ID:               checkinID,
			LockID:           lock.ID,
			RequestedAt:      time.Now(),
			Status:           "pending",
			VerificationCode: code,
		})
	}

	b.mustSaveState()
	b.pendingAction = "checkin_photo"
	b.Send(fmt.Sprintf(
		"📸 *CHECK-IN REQUERIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n🔢 *Código de verificación: `%s`*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Tienes 30 minutos para responder._",
		stripMarkdown(msg), code,
	))
}

func (b *Bot) HandleCheckinPhoto(imgBytes []byte, mime string) {
	if !b.state.PendingCheckin {
		b.pendingAction = ""
		return
	}
	b.Send("_Enviando a Chaster..._")

	lockID := b.state.CurrentLockID
	if lockID == "" {
		if lock, err := b.chaster.GetActiveLock(); err == nil {
			lockID = lock.ID
		}
	}

	if err := b.chaster.SubmitVerificationPicture(lockID, imgBytes, mime); err != nil {
		log.Printf("[checkin] error enviando foto a Chaster: %v", err)
		b.Send("❌ Error enviando la foto a Chaster. Inténtalo de nuevo.")
		return
	}

	b.pendingAction = ""
	respondedAt := time.Now()
	responseTimeMins := 0
	if b.state.CheckinExpiresAt != nil {
		responseTimeMins = int(30 - time.Until(*b.state.CheckinExpiresAt).Minutes())
		if responseTimeMins < 0 {
			responseTimeMins = 0
		}
	}

	// Subir foto a Cloudinary para estadísticas
	photoURL := ""
	if b.cloudinary != nil {
		if url, _, cerr := b.cloudinary.Upload(imgBytes, mime, "chaster/checkins"); cerr == nil {
			photoURL = url
		}
	}
	if b.db != nil && b.state.CurrentCheckinID != "" {
		b.db.UpdateCheckin(b.state.CurrentCheckinID, "submitted", photoURL, &respondedAt, responseTimeMins)
	}

	b.state.PendingCheckin = false
	b.state.CheckinExpiresAt = nil
	b.state.CurrentCheckinID = ""
	b.state.CheckinVerificationCode = ""
	b.state.TasksStreak++
	b.mustSaveState()
	b.Send("✅ *CHECK-IN ENVIADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Foto enviada a la comunidad de Chaster. +1 obediencia._")
}

// CheckObedienceDecay resta 1 punto si llevan 2+ días sin completar tarea.
func (b *Bot) CheckObedienceDecay() {
	if b.state.TasksStreak == 0 || b.state.LastTaskCompletedDate == "" {
		return
	}
	loc, _ := time.LoadLocation("America/Bogota")
	now := time.Now().In(loc)
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	today := now.Format("2006-01-02")
	last := b.state.LastTaskCompletedDate
	if last == today || last == yesterday {
		return
	}
	// Lleva 2+ días sin completar — perder 1 punto
	b.state.TasksStreak = max(0, b.state.TasksStreak-1)
	b.state.ConsecutiveDays = 0
	b.mustSaveState()
	log.Printf("[decay] obediencia reducida a %d por inactividad", b.state.TasksStreak)
}

func (b *Bot) CheckCheckinExpiry() {
	if !b.state.PendingCheckin || b.state.CheckinExpiresAt == nil {
		return
	}

	// Recordatorio a los 15 minutos restantes
	if !b.state.CheckinReminderSent {
		timeLeft := time.Until(*b.state.CheckinExpiresAt)
		if timeLeft <= 15*time.Minute && timeLeft > 0 {
			b.state.CheckinReminderSent = true
			b.mustSaveState()
			b.Send("⚠️ *CHECK-IN — 15 minutos*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Papi sigue esperando la foto. Queda poco tiempo._")
			return
		}
	}

	if time.Now().Before(*b.state.CheckinExpiresAt) {
		return
	}
	if b.db != nil && b.state.CurrentCheckinID != "" {
		now := time.Now()
		b.db.UpdateCheckin(b.state.CurrentCheckinID, "missed", "", &now, 30)
	}
	b.state.PendingCheckin = false
	b.state.CheckinExpiresAt = nil
	b.state.CheckinReminderSent = false
	b.state.CurrentCheckinID = ""
	b.state.CheckinVerificationCode = ""
	b.pendingAction = ""
	b.mustSaveState()
	b.Send("⏰ *CHECK-IN IGNORADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_No respondiste a tiempo. Consecuencias._")
	b.addWeeklyDebt("check-in ignorado — no respondió a tiempo")
	b.autoPillory("no respondió al check-in a tiempo")
}

// CheckRitualExpiry penaliza si el ritual matutino fue ignorado (llamado a las 11am)
func (b *Bot) CheckRitualExpiry() {
	if b.state.RitualStep == 0 {
		return
	}
	if _, err := b.chaster.GetActiveLock(); err != nil {
		return
	}
	loc, _ := time.LoadLocation("America/Bogota")
	if time.Now().In(loc).Hour() < 11 {
		return
	}
	// Ritual iniciado pero no completado y ya pasaron las 11am — penalizar
	b.state.RitualStep = 0
	b.state.LastRitualDate = todayStr()
	b.pendingAction = ""
	b.mustSaveState()
	b.Send("💀 *RITUAL IGNORADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_No completaste el ritual de Papi. Hay consecuencias._")
	b.addWeeklyDebt("ritual matutino ignorado")
	b.autoPillory("ignoró el ritual matutino de Papi")
}

// ── Condicionamiento ───────────────────────────────────────────────────────

func (b *Bot) SendConditioningMessage() {
	if _, err := b.chaster.GetActiveLock(); err != nil {
		return
	}
	loc, _ := time.LoadLocation("America/Bogota")
	hour := time.Now().In(loc).Hour()
	obedienceLevel := models.GetObedienceLevelFromPoints(b.state.TasksStreak)
	msg, err := b.ai.GenerateConditioningMessage(b.daysLocked(), b.state.Toys, hour, obedienceLevel)
	if err != nil {
		return
	}
	b.Send(stripMarkdown(msg))
}

// ── Juicio dominical ───────────────────────────────────────────────────────

func (b *Bot) HandleWeeklyJudgment() {
	today := todayStr()
	if b.state.LastJudgmentDate == today {
		return
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}

	b.Send("⚖️ *PAPI HACE EL RECUENTO DE LA SEMANA...*")

	verdict, err := b.ai.GenerateWeeklyJudgment(
		b.daysLocked(),
		b.state.Toys,
		b.state.WeeklyDebt,
		b.state.WeeklyDebtDetails,
		b.state.TasksCompleted,
		b.state.TasksFailed,
	)
	if err != nil {
		b.Send("❌ Error en el juicio.")
		return
	}

	b.state.LastJudgmentDate = today
	b.state.WeeklyDebt = 0
	b.state.WeeklyDebtDetails = nil
	b.mustSaveState()

	b.Send("⚖️ *SENTENCIA SEMANAL*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(verdict.Message))

	if verdict.AddTimeHours > 0 {
		if err := b.chaster.AddTime(lock.ID, verdict.AddTimeHours*3600); err != nil {
			log.Printf("[Judgment] error addtime: %v", err)
		} else {
			b.state.TotalTimeAddedHours += verdict.AddTimeHours
			b.mustSaveState()
			b.Send(fmt.Sprintf("⏳ *+%dh* añadidas a tu condena.", verdict.AddTimeHours))
		}
	}

	if verdict.PilloryMins > 0 {
		reason, _ := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, "weekly judgment")
		if strings.TrimSpace(reason) == "" {
			reason = "Weekly judgment by her keyholder"
		}
		b.chaster.PutInPillory(lock.ID, verdict.PilloryMins*60, strings.TrimSpace(reason))
		b.Send(fmt.Sprintf("⛓ *Cepo* — %d minutos.", verdict.PilloryMins))
	}

	if verdict.FreezeHours > 0 {
		b.chaster.FreezeLock(lock.ID)
		expiresAt := time.Now().Add(time.Duration(verdict.FreezeHours) * time.Hour)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "freeze", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf("❄️ *Congelada* — %dh.", verdict.FreezeHours))
	}

	if strings.TrimSpace(verdict.SpecialTask) != "" {
		b.Send(fmt.Sprintf("📋 *TAREA ESPECIAL*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_", verdict.SpecialTask))
		loc, _ := time.LoadLocation("America/Bogota")
		now := time.Now().In(loc)
		b.state.CurrentTask = &models.Task{
			ID:            fmt.Sprintf("judgment-%d", now.Unix()),
			Description:   verdict.SpecialTask,
			AssignedAt:    now,
			DueAt:         now.Add(2 * time.Hour),
			PenaltyHours:  3,
			RewardHours:   0,
			AwaitingPhoto: true,
		}
		b.mustSaveState()
	}
}

// ── Ruleta diaria ──────────────────────────────────────────────────────────

func (b *Bot) HandleRuleta() {
	if b.state.LastRuletaDate == todayStr() {
		b.Send("Ya giraste la ruleta hoy. Vuelve mañana.")
		return
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa.")
		return
	}
	b.Send("_Girando la ruleta..._")
	obedienceLevel := models.GetObedienceLevelFromPoints(b.state.TasksStreak)
	outcome, err := b.ai.SpinRuleta(b.daysLocked(), b.state.Toys, b.state.TasksCompleted, b.state.TasksFailed, obedienceLevel)
	if err != nil {
		b.Send("❌ Error girando la ruleta.")
		return
	}
	b.state.LastRuletaDate = todayStr()
	b.mustSaveState()

	switch outcome.Action {
	case "remove_time":
		if err := b.chaster.RemoveTime(lock.ID, outcome.Value*3600); err != nil {
			log.Printf("[Ruleta] error remove_time: %v", err)
		} else {
			b.state.TotalTimeRemovedHours += outcome.Value
			b.mustSaveState()
		}
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n*-%dh* quitadas de tu condena.", stripMarkdown(outcome.Message), outcome.Value))
	case "add_time":
		if err := b.chaster.AddTime(lock.ID, outcome.Value*3600); err != nil {
			log.Printf("[Ruleta] error add_time: %v", err)
		} else {
			b.state.TotalTimeAddedHours += outcome.Value
			b.mustSaveState()
		}
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas a tu condena.", stripMarkdown(outcome.Message), outcome.Value))
	case "pillory":
		mins := outcome.Value
		if mins <= 0 {
			mins = 15
		}
		reason, _ := b.ai.GeneratePilloryReason(b.daysLocked(), b.state.Toys, "ruleta")
		if strings.TrimSpace(reason) == "" {
			reason = "Roulette sent her to pillory"
		}
		b.chaster.PutInPillory(lock.ID, mins*60, strings.TrimSpace(reason))
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n⛓ Cepo por *%d minutos*.", stripMarkdown(outcome.Message), mins))
	case "freeze":
		mins := outcome.Value
		if mins <= 0 {
			mins = 60
		}
		b.chaster.FreezeLock(lock.ID)
		expiresAt := time.Now().Add(time.Duration(mins) * time.Minute)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "freeze", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n❄️ Congelada por *%d minutos*.", stripMarkdown(outcome.Message), mins))
	case "hide_time":
		mins := outcome.Value
		if mins <= 0 {
			mins = 120
		}
		b.chaster.SetTimerVisibility(lock.ID, false)
		expiresAt := time.Now().Add(time.Duration(mins) * time.Minute)
		b.state.ActiveEvent = &models.ActiveEvent{Type: "hidetime", ExpiresAt: expiresAt}
		b.mustSaveState()
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n🙈 Timer oculto por *%d minutos*.", stripMarkdown(outcome.Message), mins))
	case "extra_task":
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Una tarea extra te espera._", stripMarkdown(outcome.Message)))
		b.handleTaskInternal(models.GetIntensity(b.daysLocked()))
	default:
		b.Send(fmt.Sprintf("🎰 *RULETA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s", stripMarkdown(outcome.Message)))
	}
}

// ── Tareas comunitarias de Chaster ─────────────────────────────────────────

// HandleChasterTaskCommand asigna una nueva tarea comunitaria de Chaster.
// Genera una tarea simple en inglés, la asigna via Extensions API y pide foto al usuario.
func (b *Bot) HandleChasterTaskCommand() {
	if !b.chaster.HasExtension() {
		b.Send("❌ La extensión de Chaster no está configurada. Necesitas CHASTER_EXTENSION_TOKEN y CHASTER_EXTENSION_SLUG.")
		return
	}
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa en Chaster.")
		return
	}
	if b.state.PendingChasterTask != "" {
		b.Send(fmt.Sprintf(
			"📋 *TAREA COMUNITARIA ACTIVA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Manda la foto para completarla._",
			b.state.PendingChasterTask,
		))
		return
	}

	b.Send("_Papi está preparando una tarea para la comunidad..._")

	var recentTasks []string
	if b.db != nil {
		recentTasks, _ = b.db.GetRecentChasterTaskDescriptions(10)
	}
	taskDesc, err := b.ai.GenerateChasterTask(b.daysLocked(), b.state.Toys, recentTasks)
	if err != nil {
		b.Send("❌ Error generando la tarea.")
		return
	}
	taskDesc = strings.TrimSpace(taskDesc)
	// Limitar a 160 caracteres por restricción de Chaster
	if len(taskDesc) > 160 {
		taskDesc = taskDesc[:160]
	}

	sessionID, err := b.chaster.GetSessionByLockID(lock.ID)
	if err != nil {
		b.Send(fmt.Sprintf("❌ No se pudo obtener la sesión de extensión: %v", err))
		return
	}

	dbID := fmt.Sprintf("chatask-%d", time.Now().UnixNano())
	if b.db != nil {
		if err := b.db.SaveChasterTask(&storage.ChasterTask{
			ID: dbID, Description: taskDesc, Result: "pending", AssignedAt: time.Now(),
		}); err != nil {
			log.Printf("[ChasterTask] error guardando tarea en DB: %v", err)
		}
	}
	b.state.ChasterTaskDBID = dbID

	if err := b.chaster.AssignChasterTask(sessionID, taskDesc); err != nil {
		b.Send(fmt.Sprintf("❌ Error asignando la tarea en Chaster: %v", err))
		return
	}

	now := time.Now()
	b.state.PendingChasterTask = taskDesc
	b.state.ChasterTaskSessionID = sessionID
	b.state.ChasterTaskLockID = lock.ID
	b.state.ChasterTaskAssignedAt = &now
	b.mustSaveState()
	b.pendingAction = "chaster_task_photo"

	b.Send(fmt.Sprintf(
		"📋 *TAREA COMUNITARIA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La comunidad de Chaster la está viendo. Complétala y manda la foto._",
		taskDesc,
	))
}

// HandleChasterTaskPhoto procesa la foto de la tarea comunitaria.
// Marca la tarea como completada en Chaster y espera votación de la comunidad.
func (b *Bot) HandleChasterTaskPhoto(imgBytes []byte, mime string) {
	if b.state.PendingChasterTask == "" {
		b.pendingAction = ""
		return
	}

	b.Send("_Enviando evidencia a Chaster..._")

	// Subir foto a Cloudinary para nuestro registro
	var chataskPhotoURL string
	if b.cloudinary != nil {
		url, _, err := b.cloudinary.Upload(imgBytes, mime, "chaster/community-tasks")
		if err != nil {
			log.Printf("[ChasterTask] error subiendo foto a Cloudinary: %v", err)
		} else {
			chataskPhotoURL = url
		}
	}
	if b.db != nil && b.state.ChasterTaskDBID != "" && chataskPhotoURL != "" {
		b.db.UpdateChasterTaskResult(b.state.ChasterTaskDBID, "pending", chataskPhotoURL, nil)
	}

	// 1. Subir foto a Chaster y obtener el verificationPictureToken
	token, err := b.chaster.UploadVerificationPhoto(imgBytes, mime)
	if err != nil {
		log.Printf("[ChasterTask] error subiendo foto de verificación: %v", err)
		b.Send(fmt.Sprintf("❌ Error subiendo la foto a Chaster: %v", err))
		return
	}

	// 2. Completar la tarea con el token (user endpoint)
	lockID := b.state.ChasterTaskLockID
	if err := b.chaster.CompleteTaskWithVerification(lockID, token); err != nil {
		log.Printf("[ChasterTask] error completando tarea: %v", err)
		b.Send(fmt.Sprintf("❌ Error completando la tarea en Chaster: %v", err))
		return
	}

	// Limpiar el estado de "pendiente de foto" pero mantener lockID/assignedAt para polling
	b.state.PendingChasterTask = ""
	b.state.ChasterTaskSessionID = ""
	b.pendingAction = ""
	b.mustSaveState()

	b.Send(
		"📤 *EVIDENCIA ENVIADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" +
			"_La comunidad de Chaster está votando. Tienes hasta 6 horas para la aprobación._\n" +
			"▬▬▬▬▬▬▬▬▬▬▬▬\n" +
			"_Te aviso cuando salga el resultado._",
	)
}

// CheckChasterTaskVote verifica si hay un voto comunitario pendiente y reporta el resultado.
// Llamado por el scheduler cada 15 minutos.
func (b *Bot) CheckChasterTaskVote() {
	if b.state.ChasterTaskLockID == "" || b.state.ChasterTaskAssignedAt == nil {
		return
	}

	// Timeout: si pasaron más de 2 horas sin resultado, abandonar
	if time.Since(*b.state.ChasterTaskAssignedAt) > 6*time.Hour {
		log.Printf("[ChasterTask] timeout esperando voto — limpiando estado")
		if b.db != nil && b.state.ChasterTaskDBID != "" {
			now := time.Now()
			b.db.UpdateChasterTaskResult(b.state.ChasterTaskDBID, "timeout", "", &now)
		}
		b.clearChasterTaskState()
		b.Send("⏰ *TAREA COMUNITARIA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_El tiempo de votación expiró sin resultado definitivo._")
		return
	}

	entries, err := b.chaster.GetTaskHistory(b.state.ChasterTaskLockID)
	if err != nil {
		log.Printf("[ChasterTask] error consultando historial: %v", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	// Tomar la entrada más reciente
	latest := entries[0]

	dbID := b.state.ChasterTaskDBID

	switch latest.Status {
	case "verified":
		b.clearChasterTaskState()
		if b.db != nil && dbID != "" {
			now := time.Now()
			b.db.UpdateChasterTaskResult(dbID, "verified", "", &now)
		}
		lock, err := b.chaster.GetActiveLock()
		if err == nil {
			if err := b.chaster.RemoveTime(lock.ID, 3600); err != nil {
				log.Printf("[ChasterTask] error quitando tiempo: %v", err)
			} else {
				b.state.TotalTimeRemovedHours++
				b.mustSaveState()
			}
		}
		b.Send("✅ *COMUNIDAD APROBÓ*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La comunidad verificó tu tarea. Papi está... satisfecho._\n▬▬▬▬▬▬▬▬▬▬▬▬\n*-1h* quitada de tu condena.")

	case "rejected":
		b.clearChasterTaskState()
		if b.db != nil && dbID != "" {
			now := time.Now()
			b.db.UpdateChasterTaskResult(dbID, "rejected", "", &now)
		}
		lock, err := b.chaster.GetActiveLock()
		if err == nil {
			if err := b.chaster.AddTime(lock.ID, 3600); err != nil {
				log.Printf("[ChasterTask] error añadiendo tiempo: %v", err)
			} else {
				b.state.TotalTimeAddedHours++
				b.mustSaveState()
			}
		}
		b.addWeeklyDebt("tarea comunitaria rechazada por la comunidad de Chaster")
		b.Send("❌ *COMUNIDAD RECHAZÓ*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La comunidad no quedó satisfecha con tu evidencia. Papi tampoco._\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+1h* añadida a tu condena.")

	case "abandoned":
		b.clearChasterTaskState()
		if b.db != nil && dbID != "" {
			now := time.Now()
			b.db.UpdateChasterTaskResult(dbID, "abandoned", "", &now)
		}
		b.addWeeklyDebt("tarea comunitaria abandonada")
		b.Send("💀 *TAREA ABANDONADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La tarea fue marcada como abandonada. Consecuencias._")

	case "pending_verification":
		log.Printf("[ChasterTask] votación en curso para lock %s", b.state.ChasterTaskLockID)
	}
}

func (b *Bot) clearChasterTaskState() {
	b.state.ChasterTaskLockID = ""
	b.state.ChasterTaskAssignedAt = nil
	b.state.PendingChasterTask = ""
	b.state.ChasterTaskSessionID = ""
	b.state.ChasterTaskDBID = ""
	b.mustSaveState()
}

// handleEventNegotiation maneja un ruego para terminar un evento activo antes de tiempo
func (b *Bot) handleEventNegotiation(text string) {
	if b.state.ActiveEvent == nil {
		return
	}

	minutesRemaining := int(time.Until(b.state.ActiveEvent.ExpiresAt).Minutes())
	if minutesRemaining < 0 {
		minutesRemaining = 0
	}

	b.Send("_Evaluando tu ruego..._")

	result, err := b.ai.NegotiateActiveEvent(
		text,
		b.state.ActiveEvent.Type,
		minutesRemaining,
		b.state.Toys,
		b.daysLocked(),
		b.state.TasksCompleted,
		b.state.TasksFailed,
	)
	if err != nil {
		b.Send("_..._")
		return
	}

	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return
	}

	// Capturar referencia antes del switch para evitar nil si expira concurrentemente
	activeEvent := b.state.ActiveEvent
	if activeEvent == nil {
		return
	}

	switch result.Decision {
	case "approved":
		eventType := activeEvent.Type
		b.state.ActiveEvent = nil
		b.mustSaveState()
		switch eventType {
		case "freeze":
			b.chaster.UnfreezeLock(lock.ID)
		case "hidetime":
			b.chaster.SetTimerVisibility(lock.ID, true)
		}
		b.Send("▪️ *CONCEDIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(result.Message))

	case "rejected":
		b.Send("▪️ *RECHAZADO*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(result.Message))

	case "counter":
		b.Send(fmt.Sprintf(
			"▪️ *CONTRAOFERTA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Tarea: %s_",
			stripMarkdown(result.Message),
			stripMarkdown(result.Task),
		))

	case "penalty":
		// Extender el evento como castigo usando referencia capturada
		activeEvent.ExpiresAt = activeEvent.ExpiresAt.Add(30 * time.Minute)
		b.mustSaveState()
		b.Send("▪️ *PENALIZACIÓN*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(result.Message) + "\n▬▬▬▬▬▬▬▬▬▬▬▬\n_+30 minutos añadidos al evento._")
	}
}

// ── Contrato ───────────────────────────────────────────────────────────────

func (b *Bot) HandleContrato() {
	if b.db == nil {
		b.Send("❌ Base de datos no disponible.")
		return
	}
	c, err := b.db.GetLatestContract()
	if err != nil || c == nil {
		b.Send("📜 No hay contrato activo para esta sesión.")
		return
	}
	b.Send("📜 *CONTRATO ACTUAL*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + stripMarkdown(c.Text))
}

// ── Reset ──────────────────────────────────────────────────────────────────

// HandleDBWipe borra toda la DB, siembra el dato de orgasmo inicial y resetea el estado.
// Comando oculto — no aparece en /help.
func (b *Bot) HandleDBWipe() {
	if b.db == nil {
		b.Send("❌ DB no disponible.")
		return
	}

	b.Send("_Borrando todo..._")

	if err := b.db.ResetAllTables(); err != nil {
		b.Send(fmt.Sprintf("❌ Error limpiando DB: %v", err))
		return
	}

	// Sembrar: 1 orgasmo concedido hace 3 días
	if err := b.db.SeedOrgasmGranted(3); err != nil {
		log.Printf("[dbwipe] error sembrando orgasmo: %v", err)
	}

	// Resetear state.json
	b.stateMu.Lock()
	b.state = &models.AppState{Toys: []models.Toy{}}
	b.stateMu.Unlock()
	b.mustSaveState()

	// Resetear estado transitorio
	b.pendingAction = ""
	b.cachedDaysLocked = 0
	b.cachedDaysLockedAt = time.Time{}

	b.Send("✅ *DB reseteada.*\n▬▬▬▬▬▬▬▬▬▬▬▬\nDatos sembrados:\n— 1 orgasmo concedido hace 3 días\n\nTodo lo demás está vacío. Listo para mañana.")
}

// ── Guardarropa ────────────────────────────────────────────────────────────

func (b *Bot) HandleWardrobe(args string) {
	if b.db == nil {
		b.Send("❌ Base de datos no disponible.")
		return
	}
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	subCmd := ""
	if len(parts) > 0 {
		subCmd = strings.ToLower(parts[0])
	}

	switch subCmd {
	case "add", "agregar":
		b.pendingAction = "new_clothing"
		b.Send("👗 *NUEVA PRENDA*\n▬▬▬▬▬▬▬▬▬▬▬▬\nManda la foto de la prenda.\n_La IA generará nombre, descripción y tipo automáticamente._")

	case "remove", "quitar":
		items, err := b.db.GetClothingItems()
		if err != nil || len(items) == 0 {
			b.Send("El guardarropa está vacío.")
			return
		}
		lines := []string{"👗 *¿CUÁL QUIERES ELIMINAR?*\n▬▬▬▬▬▬▬▬▬▬▬▬"}
		for i, c := range items {
			lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, c.Name, c.Type))
		}
		lines = append(lines, "▬▬▬▬▬▬▬▬▬▬▬▬\n_Responde con el número._")
		b.Send(strings.Join(lines, "\n"))
		b.pendingAction = "removing_clothing"

	default:
		items, err := b.db.GetClothingItems()
		if err != nil || len(items) == 0 {
			b.Send("👗 *GUARDARROPA*\n\nVacío. Añade prendas con:\n`/wardrobe add`")
			return
		}
		lines := []string{"👗 *GUARDARROPA*\n"}
		for i, c := range items {
			typeIcon := clothingTypeIcon(c.Type)
			lines = append(lines, fmt.Sprintf("%d. %s%s", i+1, typeIcon, c.Name))
		}
		// Mostrar outfit del día si existe
		if b.state.DailyOutfitDesc != "" && b.state.DailyOutfitDate == todayStr() {
			status := "⏳ _esperando foto..._"
			if b.state.OutfitConfirmed {
				status = "✅ confirmado"
			}
			lines = append(lines, "\n▬▬▬▬▬▬▬▬▬▬▬▬")
			lines = append(lines, fmt.Sprintf("👗 *Outfit hoy* — %s\n_%s_", status, b.state.DailyOutfitDesc))
			if b.state.DailyPoseDesc != "" {
				lines = append(lines, fmt.Sprintf("🧍 *Pose* — _%s_", b.state.DailyPoseDesc))
			}
			if b.state.OutfitConfirmed && b.state.DailyOutfitComment != "" {
				lines = append(lines, fmt.Sprintf("💬 _%s_", b.state.DailyOutfitComment))
			}
		}
		lines = append(lines, "\n`/wardrobe add` — añadir")
		lines = append(lines, "`/wardrobe remove` — eliminar")
		b.Send(strings.Join(lines, "\n"))
	}
}

func clothingTypeIcon(t string) string {
	switch t {
	case "thong":
		return "🩲 "
	case "bra":
		return "👙 "
	case "stockings":
		return "🦵 "
	case "socks":
		return "🧦 "
	case "collar":
		return "💎 "
	case "lingerie":
		return "🌸 "
	case "dress":
		return "👗 "
	case "top":
		return "👚 "
	case "bottom":
		return "👘 "
	case "shoes":
		return "👠 "
	case "accessory":
		return "💍 "
	default:
		return "🎀 "
	}
}

// handleClothingRemoveSelection procesa la selección de prenda a eliminar
func (b *Bot) handleClothingRemoveSelection(text string) {
	items, err := b.db.GetClothingItems()
	if err != nil {
		b.pendingAction = ""
		b.Send("❌ Error obteniendo el guardarropa.")
		return
	}
	var num int
	fmt.Sscanf(strings.TrimSpace(text), "%d", &num)
	if num < 1 || num > len(items) {
		lines := []string{"❌ Número inválido. ¿Cuál quieres eliminar?"}
		for i, c := range items {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, c.Name))
		}
		b.Send(strings.Join(lines, "\n"))
		return
	}
	selected := items[num-1]
	b.db.DeleteClothingItem(selected.ID)
	b.pendingAction = ""
	b.Send(fmt.Sprintf("🗑 *%s* eliminada del guardarropa.", selected.Name))
}

// HandleWardrobePhoto procesa la foto de una prenda nueva: IA analiza, sube a Cloudinary y guarda en DB
func (b *Bot) HandleWardrobePhoto(imgBytes []byte, mimeType string) {
	b.pendingAction = ""

	b.Send("_Analizando la prenda..._")

	info, err := b.ai.DescribeClothing(imgBytes, mimeType)
	if err != nil || info == nil {
		b.Send("❌ Error analizando la foto.")
		return
	}

	var photoURL string
	if b.cloudinary != nil {
		url, _, err := b.cloudinary.Upload(imgBytes, mimeType, "chaster/wardrobe")
		if err != nil {
			log.Printf("error subiendo foto de prenda: %v", err)
		} else {
			photoURL = url
		}
	}

	itemID := fmt.Sprintf("clothing-%d", time.Now().UnixNano())
	if err := b.db.SaveClothingItem(&storage.ClothingItem{
		ID:          itemID,
		Name:        info.Name,
		Description: info.Description,
		PhotoURL:    photoURL,
		Type:        info.Type,
		AddedAt:     time.Now(),
	}); err != nil {
		log.Printf("error guardando prenda en DB: %v", err)
		b.Send("❌ Error guardando la prenda.")
		return
	}

	b.Send(fmt.Sprintf(
		"✅ *%s* añadida al guardarropa.\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_",
		info.Name, info.Description,
	))
}

// SendDailyOutfit asigna el outfit del día (llamado por el scheduler a las 10am)
func (b *Bot) SendDailyOutfit() {
	if b.state.DailyOutfitDate == todayStr() {
		return // ya asignado hoy
	}
	if _, err := b.chaster.GetActiveLock(); err != nil {
		return // sin lock activo
	}
	if b.db == nil {
		return
	}
	items, err := b.db.GetClothingItems()
	if err != nil || len(items) == 0 {
		return // sin prendas registradas
	}

	// Construir lista de nombres para la IA
	wardrobeList := make([]string, 0, len(items))
	for _, c := range items {
		wardrobeList = append(wardrobeList, fmt.Sprintf("%s (%s)", c.Name, c.Type))
	}

	intensity := models.GetIntensity(b.daysLocked())
	assignment, err := b.ai.GenerateOutfitAssignment(b.daysLocked(), wardrobeList, intensity)
	if err != nil {
		log.Printf("[outfit] error generando outfit: %v", err)
		return
	}

	b.state.DailyOutfitDesc = assignment.Description
	b.state.DailyPoseDesc = assignment.Pose
	b.state.DailyOutfitDate = todayStr()
	b.state.OutfitConfirmed = false
	b.state.DailyOutfitComment = ""
	b.mustSaveState()
	b.pendingAction = "outfit_photo"

	b.Send(fmt.Sprintf(
		"👗 *OUTFIT DEL DÍA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Cuando estés lista, manda la foto._",
		stripMarkdown(assignment.Message),
	))
}

// HandleOutfitPhoto acepta la foto del outfit y genera el comentario de Papi
func (b *Bot) HandleOutfitPhoto(imgBytes []byte, mimeType string) {
	if b.state.DailyOutfitDesc == "" || b.state.OutfitConfirmed {
		b.pendingAction = ""
		return
	}

	// Subir foto a Cloudinary
	var outfitPhotoURL string
	if b.cloudinary != nil {
		url, _, err := b.cloudinary.Upload(imgBytes, mimeType, "chaster/outfits")
		if err != nil {
			log.Printf("error subiendo foto de outfit: %v", err)
		} else {
			outfitPhotoURL = url
		}
	}

	b.pendingAction = ""
	b.state.OutfitConfirmed = true
	b.state.DailyOutfitPhotoURL = outfitPhotoURL

	// Generar comentario de Papi
	comment, err := b.ai.GenerateOutfitComment(b.daysLocked(), b.state.DailyOutfitDesc, b.state.DailyPoseDesc)
	if err != nil {
		comment = "Perfecta. Así te quiero todo el día."
	}
	b.state.DailyOutfitComment = strings.TrimSpace(comment)
	b.mustSaveState()

	// Guardar en historial DB
	if b.db != nil {
		loc, _ := time.LoadLocation("America/Bogota")
		b.db.SaveOutfitEntry(&storage.OutfitEntry{
			ID:         fmt.Sprintf("outfit-%d", time.Now().UnixNano()),
			Date:       time.Now().In(loc).Format("2006-01-02"),
			OutfitDesc: b.state.DailyOutfitDesc,
			PoseDesc:   b.state.DailyPoseDesc,
			PhotoURL:   outfitPhotoURL,
			Comment:    b.state.DailyOutfitComment,
			CreatedAt:  time.Now(),
		})
	}

	b.Send(fmt.Sprintf(
		"✅ *OUTFIT DEL DÍA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s",
		stripMarkdown(b.state.DailyOutfitComment),
	))
}
