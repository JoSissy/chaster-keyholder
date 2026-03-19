package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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

	// Flujo de agregar juguete con foto
	awaitingToyPhoto bool
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
	b.state = b.loadState()
	// Cargar juguetes desde DB
	if toys, err := db.GetToys(); err == nil {
		b.state.Toys = []models.Toy{}
		for _, t := range toys {
			b.state.Toys = append(b.state.Toys, models.Toy{
				ID: t.ID, Name: t.Name,
				Description: t.Description, PhotoURL: t.PhotoURL,
				AddedAt: t.CreatedAt,
			})
		}
	}
	return b, nil
}

// ── Estado ─────────────────────────────────────────────────────────────────

func (b *Bot) loadState() *models.AppState {
	data, err := os.ReadFile(b.statePath)
	if err != nil {
		return &models.AppState{
			Toys: []models.Toy{},
		}
	}
	var s models.AppState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("error parseando state.json: %v — usando estado vacío", err)
		return &models.AppState{Toys: []models.Toy{}}
	}
	if s.Toys == nil {
		s.Toys = []models.Toy{}
	}
	return &s
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

	// Si el lock está listo para desbloquear, ejecutarlo automáticamente
	if lock.IsReadyToUnlock {
		b.Send("🔓 *LISTA PARA DESBLOQUEAR*\n▬▬▬▬▬▬▬▬▬▬▬▬\n_Ejecutando desbloqueo automático..._")
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

	frozenStr := ""
	if lock.Frozen {
		frozenStr = "\n❄️ Estado — *CONGELADA*"
	}

	taskStatus := "sin asignar"
	if b.state.CurrentTask != nil {
		if b.state.CurrentTask.Completed {
			taskStatus = "completada"
		} else if b.state.CurrentTask.Failed {
			taskStatus = "fallida"
		} else if b.state.CurrentTask.AwaitingPhoto {
			taskStatus = "_esperando evidencia..._"
		} else {
			taskStatus = fmt.Sprintf("⏳ _%s_", b.state.CurrentTask.Description)
		}
	}

	msg := fmt.Sprintf(
		"▪️ *ESTADO DE CONDENA*\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"⏱ Encerrada — *%dd %dh %dm*\n"+
			"⌛ Restante — *%s*\n"+
			"🌡 Nivel — *%s*\n"+
			"🧸 Juguetes — *%d*\n"+
			"📊 Balance — *+%dh / -%dh*%s\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"📋 Tarea — %s",
		days, hours, mins,
		timeRemaining,
		intensity.String(),
		len(b.state.Toys),
		b.state.TotalTimeAddedHours,
		b.state.TotalTimeRemovedHours,
		frozenStr,
		taskStatus,
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
				"✅ Recompensa — *-%dh*\n"+
				"💀 Consecuencia — *+%dh*%s",
			b.state.CurrentTask.Description,
			b.state.CurrentTask.DueAt.Format("15:04"),
			b.state.CurrentTask.RewardHours,
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

	taskDesc, err := b.ai.GenerateDailyTask(days, b.state.Toys, intensity)
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
	rewardHours := 1

	b.state.CurrentTask = &models.Task{
		ID:            fmt.Sprintf("task-%d", now.Unix()),
		Description:   taskDesc,
		AssignedAt:    now,
		DueAt:         now.Add(1 * time.Hour),
		PenaltyHours:  penaltyHours,
		RewardHours:   rewardHours,
		AwaitingPhoto: true,
	}
	b.mustSaveState()

	b.Send(fmt.Sprintf(
		"▪️ *NUEVA ORDEN* — nivel %s\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"_%s_\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"⏰ Límite — *%s*\n"+
			"✅ Recompensa — *-%dh*\n"+
			"💀 Consecuencia — *+%dh*\n\n"+
			"_Manda la foto cuando termines._",
		intensity.String(),
		taskDesc,
		b.state.CurrentTask.DueAt.Format("15:04"),
		rewardHours,
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

	b.Send("_Analizando evidencia..._")

	verdict, err := b.ai.VerifyTaskPhoto(
		imageBytes, mimeType,
		b.state.CurrentTask.Description,
		b.state.Toys,
		b.daysLocked(),
	)
	if err != nil {
		b.Send("❌ Error analizando la foto. Inténtalo de nuevo.")
		return
	}

	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No hay sesión activa en Chaster.")
		return
	}

	switch verdict.Status {
	case "approved":
		rewardHours := b.state.CurrentTask.RewardHours

		// Subir foto a Cloudinary
		var photoURL string
		if b.cloudinary != nil {
			url, cerr := b.cloudinary.Upload(imageBytes, mimeType, "chaster/tasks")
			if cerr != nil {
				log.Printf("error subiendo foto de tarea: %v", cerr)
			} else {
				photoURL = url
			}
		}

		b.state.CurrentTask.Completed = true
		b.state.CurrentTask.AwaitingPhoto = false
		b.state.TotalTimeRemovedHours += rewardHours
		b.state.TasksCompleted++
		b.state.TasksStreak++
		b.mustSaveState()

		// Guardar en DB
		if b.db != nil {
			now := time.Now()
			b.db.SaveTask(&storage.Task{
				ID: b.state.CurrentTask.ID, LockID: b.state.CurrentLockID,
				Description: b.state.CurrentTask.Description, PhotoURL: photoURL,
				AssignedAt: b.state.CurrentTask.AssignedAt, DueAt: b.state.CurrentTask.DueAt,
				CompletedAt: &now, Status: "completed",
				PenaltyHours: b.state.CurrentTask.PenaltyHours, RewardHours: rewardHours,
			})
		}

		if err := b.chaster.RemoveTime(lock.ID, rewardHours*3600); err != nil {
			log.Printf("error quitando tiempo en Chaster: %v", err)
		}

		aiMsg, _ := b.ai.GenerateTaskReward(rewardHours, b.state.Toys, b.daysLocked())
		b.Send(fmt.Sprintf(
			"✅ *EVIDENCIA APROBADA*\n\n%s\n\n_%s_\n\n_Se quitaron %dh de tu condena._",
			aiMsg, verdict.Reason, rewardHours,
		))

	case "retry":
		b.Send(fmt.Sprintf(
			"⚠️ *CASI — INTÉNTALO DE NUEVO*\n\n_%s_\n\nManda otra foto corrigiendo eso.",
			verdict.Reason,
		))

	case "rejected":
		penaltyHours := b.state.CurrentTask.PenaltyHours
		b.state.CurrentTask.Failed = true
		b.state.CurrentTask.AwaitingPhoto = false
		b.state.TotalTimeAddedHours += penaltyHours
		b.state.TasksStreak = 0
		b.mustSaveState()

		// Guardar en DB
		if b.db != nil {
			b.db.SaveTask(&storage.Task{
				ID: b.state.CurrentTask.ID, LockID: b.state.CurrentLockID,
				Description: b.state.CurrentTask.Description,
				AssignedAt:  b.state.CurrentTask.AssignedAt, DueAt: b.state.CurrentTask.DueAt,
				Status:       "failed",
				PenaltyHours: penaltyHours, RewardHours: b.state.CurrentTask.RewardHours,
			})
		}

		if err := b.chaster.AddTime(lock.ID, penaltyHours*3600); err != nil {
			log.Printf("error añadiendo tiempo en Chaster: %v", err)
		}

		aiMsg, _ := b.ai.GenerateTaskPenalty(penaltyHours, verdict.Reason)
		b.Send(fmt.Sprintf(
			"▪️ *EVIDENCIA RECHAZADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n\n*+%dh* añadidas a tu condena.",
			aiMsg, verdict.Reason, penaltyHours,
		))
	}
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
	b.mustSaveState()

	if lock, err := b.chaster.GetActiveLock(); err == nil {
		if err := b.chaster.AddTime(lock.ID, penaltyHours*3600); err != nil {
			log.Printf("error añadiendo tiempo en Chaster: %v", err)
		}
	}

	msg, _ := b.ai.GenerateTaskPenalty(penaltyHours, "confesó que no pudo completar la tarea")
	b.Send("▪️ *TAREA ABANDONADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + msg + fmt.Sprintf("\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas.", penaltyHours))
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

	newToys := []models.Toy{}
	for _, t := range b.state.Toys {
		if t.ID != selected.ID {
			newToys = append(newToys, t)
		}
	}
	b.state.Toys = newToys
	b.state.PendingToyMime = ""
	b.mustSaveState()

	b.Send(fmt.Sprintf("🗑 *%s* eliminado.", selected.Name))
}

// handleCageSelection procesa la selección de jaula durante el flujo de newlock
func (b *Bot) handleCageSelection(text string) {
	if b.db == nil {
		b.state.PendingToyMime = ""
		b.startNewLockFlow()
		return
	}

	cages, err := b.db.GetCages()
	if err != nil || len(cages) == 0 {
		b.state.PendingToyMime = ""
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
	b.state.PendingToyMime = ""
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
	b.awaitingToyPhoto = false
	hint := b.state.PendingToyMime // nombre hint que dio el usuario
	b.state.PendingToyMime = ""

	b.Send("_Analizando el juguete..._")

	// IA genera nombre y descripción
	toyInfo, err := b.ai.DescribeToy(imageBytes, mimeType, hint)
	if err != nil || toyInfo == nil {
		b.Send("❌ Error analizando la foto del juguete.")
		return
	}

	// Subir foto a Cloudinary
	var photoURL string
	if b.cloudinary != nil {
		url, err := b.cloudinary.Upload(imageBytes, mimeType, "chaster/toys")
		if err != nil {
			log.Printf("error subiendo foto de juguete: %v", err)
		} else {
			photoURL = url
		}
	}

	// Generar ID único
	toyID := fmt.Sprintf("toy-%d", time.Now().UnixNano())

	// Guardar en DB
	if b.db != nil {
		b.db.SaveToy(&storage.Toy{
			ID: toyID, Name: toyInfo.Name,
			Description: toyInfo.Description,
			PhotoURL:    photoURL,
			Type:        toyInfo.Type,
			CreatedAt:   time.Now(),
		})
	}

	// Añadir al estado en memoria
	b.state.Toys = append(b.state.Toys, models.Toy{
		ID: toyID, Name: toyInfo.Name,
		Description: toyInfo.Description,
		PhotoURL:    photoURL,
		Type:        toyInfo.Type,
		AddedAt:     time.Now(),
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

	textLower := strings.ToLower(text)

	// Detectar selección de jaula durante flujo de newlock
	if b.state.PendingToyMime == "selecting_cage" {
		b.handleCageSelection(text)
		return
	}

	// Detectar selección de juguete a eliminar
	if b.state.PendingToyMime == "removing_toy" {
		b.handleToyRemoveSelection(text)
		return
	}

	// Detectar ruegos sobre evento activo (freeze/hidetime)
	if b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt) {
		eventKeywords := map[string][]string{
			"freeze":   {"descongela", "unfreeze", "congela", "frío", "fría", "congelada", "libérame", "liberame"},
			"hidetime": {"timer", "tiempo", "cuánto", "cuanto", "falta", "muéstrame", "muestrame", "ver el tiempo"},
		}
		for _, kw := range eventKeywords[b.state.ActiveEvent.Type] {
			if strings.Contains(textLower, kw) {
				b.handleEventNegotiation(text)
				return
			}
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
	response, err := b.ai.Chat(
		text,
		b.state.Toys,
		b.daysLocked(),
		b.state.TasksCompleted,
		b.state.TasksFailed,
		b.state.TotalTimeAddedHours,
		locked,
	)
	if err != nil {
		b.Send("_..._")
		return
	}
	b.Send(stripMarkdown(response))
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
		// Pedir foto directamente — la IA genera todo
		b.state.PendingToyPhoto = nil
		b.state.PendingToyMime = "new_toy"
		b.awaitingToyPhoto = true
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
		b.state.PendingToyMime = "removing_toy"
		b.mustSaveState()

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
			case "vibrator":
				typeStr = " 📳"
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
	extStatus := "no configurada"
	if b.chaster.HasExtension() {
		extStatus = "activa ✅"
	}

	b.Send(fmt.Sprintf(`🔒 *CHASTER KEYHOLDER BOT*

*Sesión:*
/newlock — Crear nueva sesión 🔒
/status — Estado actual
/task — Ver o solicitar tarea
/explain — Cómo completar la tarea actual 📸
/fail — Confesar que fallaste 💀
/stats — Estadísticas históricas 📊

*Control avanzado* (extensión: %s):
/freeze — Congelar lock ❄️
/unfreeze — Descongelar lock 🔥
/hidetime — Ocultar tiempo restante 🙈
/showtime — Mostrar tiempo restante 👁
/pillory [min] [razón] — Enviar al cepo ⛓

*Inventario:*
/toys — Ver juguetes
/toys add [nombre] — Añadir juguete
/toys remove [nombre] — Eliminar juguete

_Para completar una tarea: manda la foto directo al chat._`, extStatus))
}

// ── Loop principal ─────────────────────────────────────────────────────────

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	// Keyboard sin duplicados, organizado
	keyboard := [][]tgbotapi.KeyboardButton{
		{tgbotapi.NewKeyboardButton("/status"), tgbotapi.NewKeyboardButton("/task")},
		{tgbotapi.NewKeyboardButton("/order"), tgbotapi.NewKeyboardButton("/fail")},
		{tgbotapi.NewKeyboardButton("/explain"), tgbotapi.NewKeyboardButton("/newlock")},
		{tgbotapi.NewKeyboardButton("/toys"), tgbotapi.NewKeyboardButton("/stats")},
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
			} else if b.awaitingToyPhoto {
				b.deleteMessage(msgID)
				b.HandleToyPhoto(imgBytes, mime)
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
		case text == "/toys":
			b.HandleToys("")
		case strings.HasPrefix(text, "/toys "):
			b.HandleToys(strings.TrimPrefix(text, "/toys "))
		// Comandos de extensión — usan el lock activo automáticamente
		case text == "/freeze":
			b.HandleFreeze()
		case text == "/unfreeze":
			b.HandleUnfreeze()
		case text == "/hidetime":
			b.HandleHideTime()
		case text == "/showtime":
			b.HandleShowTime()
		case strings.HasPrefix(text, "/pillory "):
			b.parsePilloryCommand(strings.TrimPrefix(text, "/pillory "))
		case text == "/pillory":
			b.Send("Uso: `/pillory [minutos] [razón opcional]`\nEjemplo: `/pillory 30 por no obedecer`")
		case text == "/testevent":
			b.HandleRandomEventTest()
		case text == "/testremove":
			b.HandleTestRemoveTime("")
		case strings.HasPrefix(text, "/testremove "):
			b.HandleTestRemoveTime(strings.TrimPrefix(text, "/testremove "))
		case text == "/testmsg":
			b.SendRandomMessageTest()
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
		b.state.TasksStreak = 0
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
				b.state.PendingToyMime = "selecting_cage"
				b.mustSaveState()
				return
			}
		}
	}

	b.startNewLockFlow()
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
	b.state.Toys = []models.Toy{}
	for _, t := range toys {
		b.state.Toys = append(b.state.Toys, models.Toy{
			ID: t.ID, Name: t.Name, Description: t.Description,
			PhotoURL: t.PhotoURL, Type: t.Type, InUse: t.InUse,
			AddedAt: t.CreatedAt,
		})
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
		b.Send("❌ Error creando el lock en Chaster.")
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
	}

	b.state.CurrentLockID = ""
	b.mustSaveState()
}

// CheckLockFinished verifica si el lock específico terminó.
// IMPORTANTE: Chaster NO hace unlock automático al vencer el tiempo.
// El usuario debe abrir la app y presionar desbloquear manualmente.
// Flujo:
//  1. endDate vencido + status "locked" → avisar una sola vez (flag "notified:")
//  2. status "unlocked" → mandar combinación y archivar
func (b *Bot) CheckLockFinished() {
	if b.state.CurrentLockID == "" {
		return
	}

	// Extraer lockID real (puede tener prefijo "notified:" si ya se avisó)
	rawID := b.state.CurrentLockID
	alreadyNotified := strings.HasPrefix(rawID, "notified:")
	lockID := strings.TrimPrefix(rawID, "notified:")

	lock, err := b.chaster.GetLockByID(lockID)
	if err != nil {
		// ErrLockNotFound = Chaster devolvió 404 — el lock fue desbloqueado/archivado
		// En ese caso ejecutamos el cierre aunque no podamos leer el status
		if errors.Is(err, chaster.ErrLockNotFound) {
			log.Printf("[CheckLockFinished] lock %s devolvió 404 — ejecutando finishLock", lockID)
			b.state.CurrentLockID = lockID
			b.finishLock(lockID)
			return
		}
		log.Printf("[CheckLockFinished] error consultando lock %s: %v", lockID, err)
		return
	}

	// Caso 1: tiempo vencido pero el usuario aún no hizo el unlock manual en Chaster
	if lock.Status == "locked" && lock.EndDate != nil && time.Now().After(*lock.EndDate) {
		if !alreadyNotified {
			b.Send("🔓 *TIEMPO CUMPLIDO*\n▬▬▬▬▬▬▬▬▬▬▬▬\nAbre Chaster y presiona desbloquear.\nCuando lo confirmes, te mando la combinación.")
			b.state.CurrentLockID = "notified:" + lockID
			b.mustSaveState()
		}
		return
	}

	// Caso 2: status unlocked explícito
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
	now := time.Now().In(loc)
	hour := now.Hour()

	days := b.daysLocked()
	prob := probabilidadEvento(hour, days)
	if prob == 0 {
		return
	}

	// Tirar el dado — distribución uniforme 0-99
	nano := now.UnixNano()
	if nano < 0 {
		nano = -nano
	}
	roll := int(nano % 100)
	if roll >= prob {
		return
	}

	lock, err := b.chaster.GetActiveLock()
	if err != nil {
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

// SendRandomMessage manda un mensaje espontáneo del keyholder.
// Llamado por el scheduler en horario activo.
// Si hay lock activo, mensaje de control. Si no, mensaje incentivando a encerrarse.
func (b *Bot) SendRandomMessage() {
	_, lockErr := b.chaster.GetActiveLock()
	locked := lockErr == nil

	hasActive := b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt)
	activeType := ""
	if hasActive && b.state.ActiveEvent != nil {
		activeType = b.state.ActiveEvent.Type
	}

	msg, err := b.ai.GenerateRandomMessage(
		b.daysLocked(),
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		hasActive,
		activeType,
		locked,
	)
	if err != nil {
		log.Printf("[SendRandomMessage] error: %v", err)
		return
	}

	b.Send(stripMarkdown(msg))
}

// HandleTestRemoveTime quita N horas de la condena — /testremove [horas]
func (b *Bot) HandleTestRemoveTime(args string) {
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
	b.Send(fmt.Sprintf("🧪 *TEST* — Se quitaron *%dh* de tu condena.", hours))
}

// SendRandomMessageTest fuerza un mensaje random — solo para testing con /testmsg
func (b *Bot) SendRandomMessageTest() {
	// Rate limiting — evitar múltiples ejecuciones seguidas
	b.chatMu.Lock()
	if time.Since(b.lastChatTime) < 5*time.Second {
		b.chatMu.Unlock()
		return
	}
	b.lastChatTime = time.Now()
	b.chatMu.Unlock()

	_, lockErr := b.chaster.GetActiveLock()
	locked := lockErr == nil

	hasActive := b.state.ActiveEvent != nil && time.Now().Before(b.state.ActiveEvent.ExpiresAt)
	activeType := ""
	if hasActive && b.state.ActiveEvent != nil {
		activeType = b.state.ActiveEvent.Type
	}

	msg, err := b.ai.GenerateRandomMessage(
		b.daysLocked(),
		b.state.Toys,
		b.state.TasksCompleted,
		b.state.TasksFailed,
		hasActive,
		activeType,
		locked,
	)
	if err != nil {
		b.Send(fmt.Sprintf("❌ Error: %v", err))
		return
	}

	b.Send(stripMarkdown(msg))
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
