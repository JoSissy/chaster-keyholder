package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"chaster-keyholder/ai"
	"chaster-keyholder/chaster"
	"chaster-keyholder/models"
)

type Bot struct {
	api       *tgbotapi.BotAPI
	chatID    int64
	chaster   *chaster.Client
	ai        *ai.Client
	state     *models.AppState
	statePath string
}

func NewBot(token string, chatID int64, chasterClient *chaster.Client, aiClient *ai.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	b := &Bot{
		api:       api,
		chatID:    chatID,
		chaster:   chasterClient,
		ai:        aiClient,
		statePath: "state.json",
	}
	b.state = b.loadState()
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
	json.Unmarshal(data, &s)
	if s.Toys == nil {
		s.Toys = []models.Toy{}
	}
	return &s
}

func (b *Bot) saveState() {
	data, _ := json.MarshalIndent(b.state, "", "  ")
	os.WriteFile(b.statePath, data, 0644)
}

// ── Mensajes ───────────────────────────────────────────────────────────────

func (b *Bot) Send(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := b.api.Send(msg); err != nil {
		// Si falla por Markdown inválido, reenviar sin formato
		log.Printf("error enviando mensaje con Markdown: %v — reintentando sin formato", err)
		plain := tgbotapi.NewMessage(b.chatID, stripMarkdown(text))
		b.api.Send(plain)
	}
}

// stripMarkdown elimina caracteres de formato para fallback sin parseo
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
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		return b.state.DaysLocked
	}
	days := int(time.Since(lock.StartDate).Hours()) / 24
	b.state.DaysLocked = days
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

// ── Comandos ───────────────────────────────────────────────────────────────

func (b *Bot) HandleStatus() {
	lock, err := b.chaster.GetActiveLock()
	if err != nil {
		b.Send("❌ No se encontró sesión activa en Chaster.")
		return
	}

	// Calcular tiempo real desde inicio del lock actual
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
			"📊 Balance — *+%dh / -%dh*\n"+
			"▬▬▬▬▬▬▬▬▬▬▬▬\n"+
			"📋 Tarea — %s",
		days, hours, mins,
		timeRemaining,
		intensity.String(),
		len(b.state.Toys),
		b.state.TotalTimeAdded/60,
		b.state.TotalTimeRemoved/60,
		taskStatus,
	)
	b.Send(msg)
}

func (b *Bot) HandleTask() {
	b.handleTaskInternal(models.IntensityLevel(0)) // 0 = usar intensidad automática
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
			b.state.CurrentTask.Reward,
			b.state.CurrentTask.Penalty,
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

	b.state.CurrentTask = &models.Task{
		ID:            fmt.Sprintf("task-%d", now.Unix()),
		Description:   taskDesc,
		AssignedAt:    now,
		DueAt:         now.Add(1 * time.Hour),
		Penalty:       1 + int(intensity),
		Reward:        1,
		AwaitingPhoto: true,
	}
	b.saveState()

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
		b.state.CurrentTask.Reward,
		b.state.CurrentTask.Penalty,
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
		reward := b.state.CurrentTask.Reward
		b.state.CurrentTask.Completed = true
		b.state.CurrentTask.AwaitingPhoto = false
		b.state.TotalTimeRemoved += reward
		b.state.TasksCompleted++
		b.saveState()

		b.chaster.AddTime(lock.ID, -reward*3600)

		aiMsg, _ := b.ai.GenerateTaskReward(reward, b.state.Toys, b.daysLocked()) // reward en horas
		b.Send(fmt.Sprintf(
			"✅ *EVIDENCIA APROBADA*\n\n%s\n\n_%s_\n\n_Se quitaron %d minutos de tu condena._",
			aiMsg, verdict.Reason, reward,
		))

	case "retry":
		// No penalizar, dar otra oportunidad
		b.Send(fmt.Sprintf(
			"⚠️ *CASI — INTÉNTALO DE NUEVO*\n\n_%s_\n\nManda otra foto corrigiendo eso.",
			verdict.Reason,
		))
		// AwaitingPhoto sigue en true, la tarea sigue activa

	case "rejected":
		penalty := b.state.CurrentTask.Penalty
		b.state.CurrentTask.Failed = true
		b.state.CurrentTask.AwaitingPhoto = false
		b.state.TotalTimeAdded += penalty
		b.saveState()

		b.chaster.AddTime(lock.ID, penalty*3600)

		aiMsg, _ := b.ai.GenerateTaskPenalty(penalty, verdict.Reason)
		b.Send(fmt.Sprintf(
			"▪️ *EVIDENCIA RECHAZADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\n_%s_\n\n*+%dh* añadidas a tu condena.",
			aiMsg, verdict.Reason, penalty,
		))
	}
}

func (b *Bot) HandleFail() {
	if b.state.CurrentTask == nil || b.state.CurrentTask.Completed || b.state.CurrentTask.Failed {
		b.Send("No hay tarea pendiente.")
		return
	}

	penalty := b.state.CurrentTask.Penalty
	b.state.CurrentTask.Failed = true
	b.state.CurrentTask.AwaitingPhoto = false
	b.state.TotalTimeAdded += penalty
	b.saveState()

	// Añadir tiempo solo si hay lock activo
	if lock, err := b.chaster.GetActiveLock(); err == nil {
		b.chaster.AddTime(lock.ID, penalty*3600)
	}

	msg, _ := b.ai.GenerateTaskPenalty(penalty, "confesó que no pudo completar la tarea")
	b.Send("▪️ *TAREA ABANDONADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n" + msg + fmt.Sprintf("\n▬▬▬▬▬▬▬▬▬▬▬▬\n*+%dh* añadidas.", penalty))
}

// ── Chat libre ────────────────────────────────────────────────────────────

func (b *Bot) HandleChat(text string) {
	// Detectar si es una petición de negociación de tiempo
	negotiationKeywords := []string{
		"quitar", "reducir", "menos tiempo", "recompensa", "me porté",
		"porte bien", "negociar", "tiempo", "horas", "minutos", "liberar",
		"permiso", "puedo", "déjame", "déjame", "por favor",
	}

	isNegotiation := false
	textLower := strings.ToLower(text)
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

	// Chat libre normal
	response, err := b.ai.Chat(
		text,
		b.state.Toys,
		b.daysLocked(),
		b.state.TasksCompleted,
		b.state.TasksFailed,
		b.state.TotalTimeAdded/60,
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
		b.state.TotalTimeAdded/60,
	)
	if err != nil {
		b.Send("_..._")
		return
	}

	lock, _ := b.chaster.GetActiveLock()

	switch result.Decision {
	case "approved":
		if result.TimeHours < 0 && lock != nil {
			b.chaster.AddTime(lock.ID, result.TimeHours*3600)
			b.state.TotalTimeRemoved += -result.TimeHours * 60
			b.saveState()
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
			b.chaster.AddTime(lock.ID, result.TimeHours*3600)
			b.state.TotalTimeAdded += result.TimeHours * 60
			b.saveState()
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
	toyName := ""
	if len(parts) > 1 {
		toyName = strings.TrimSpace(parts[1])
	}

	switch subCmd {

	case "add", "agregar":
		if toyName == "" {
			b.Send("Uso: `/toys add [nombre]`\nEjemplo: `/toys add plug mediano`")
			return
		}
		b.state.Toys = append(b.state.Toys, models.Toy{
			Name:    toyName,
			AddedAt: time.Now(),
		})
		b.saveState()
		b.Send(fmt.Sprintf("✅ *%s* añadido al inventario.", toyName))

	case "remove", "quitar":
		if toyName == "" {
			b.Send("Uso: `/toys remove [nombre]`")
			return
		}
		found := false
		newToys := []models.Toy{}
		for _, t := range b.state.Toys {
			if strings.EqualFold(t.Name, toyName) {
				found = true
			} else {
				newToys = append(newToys, t)
			}
		}
		if !found {
			b.Send(fmt.Sprintf("❌ No encontré *%s* en el inventario.", toyName))
			return
		}
		b.state.Toys = newToys
		b.saveState()
		b.Send(fmt.Sprintf("🗑 *%s* eliminado.", toyName))

	default:
		if len(b.state.Toys) == 0 {
			b.Send("🧸 *INVENTARIO*\n\nVacío. Añade juguetes con:\n`/toys add [nombre]`")
			return
		}
		lines := []string{"🧸 *INVENTARIO*\n"}
		for i, t := range b.state.Toys {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, t.Name))
		}
		lines = append(lines, "\n_Comandos:_")
		lines = append(lines, "`/toys add [nombre]`")
		lines = append(lines, "`/toys remove [nombre]`")
		b.Send(strings.Join(lines, "\n"))
	}
}

// ── Minijuegos ─────────────────────────────────────────────────────────────

func (b *Bot) HandleHelp() {
	b.Send(`🔒 *CHASTER KEYHOLDER BOT*

*Sesión:*
/newlock — Crear nueva sesión de castidad 🔒
/status — Estado actual
/task — Ver o solicitar tarea
/fail — Confesar que fallaste 💀

*Inventario:*
/toys — Ver juguetes
/toys add [nombre] — Añadir juguete
/toys remove [nombre] — Eliminar juguete

_Para completar una tarea: manda la foto de evidencia directo al chat. Se borrará automáticamente._`)
}

// ── Loop principal ─────────────────────────────────────────────────────────

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	keyboard := [][]tgbotapi.KeyboardButton{
		{tgbotapi.NewKeyboardButton("/status"), tgbotapi.NewKeyboardButton("/task")},
		{tgbotapi.NewKeyboardButton("/order"), tgbotapi.NewKeyboardButton("/fail")},
		{tgbotapi.NewKeyboardButton("/fail"), tgbotapi.NewKeyboardButton("/newlock")},
		{tgbotapi.NewKeyboardButton("/toys"), tgbotapi.NewKeyboardButton("/help")},
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
				// Foto para crear nuevo lock
				b.HandleLockPhoto(imgBytes, mime, msgID)
			} else {
				// Foto de evidencia de tarea — también borrar
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
		case text == "/newlock":
			b.HandleNewLock("")
		case strings.HasPrefix(text, "/newlock "):
			b.HandleNewLock(strings.TrimPrefix(text, "/newlock "))
		case text == "/help":
			b.HandleHelp()
		case text == "/toys":
			b.HandleToys("")
		case strings.HasPrefix(text, "/toys "):
			b.HandleToys(strings.TrimPrefix(text, "/toys "))
		case text != "" && !strings.HasPrefix(text, "/"):
			// Mensaje libre → chat con el keyholder
			b.HandleChat(text)
		}
	}
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

	if b.state.CurrentTask != nil && !b.state.CurrentTask.Completed && !b.state.CurrentTask.Failed {
		penalty := b.state.CurrentTask.Penalty
		b.chaster.AddTime(lock.ID, penalty*3600)
		b.state.CurrentTask.Failed = true
		b.state.TotalTimeAdded += penalty * 60
	}

	msg, _ := b.ai.GenerateNightMessage(days, taskCompleted, b.state.Toys)
	b.Send("🌙 *BUENAS NOCHES*\n\n" + msg)

	b.state.CurrentTask = nil
	b.saveState()
}

// ── Nuevo lock ─────────────────────────────────────────────────────────────

// parseDuration parsea strings como "4 horas", "1 minuto", "2 dias" a segundos
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
	case strings.HasPrefix(unit, "minute") || strings.HasPrefix(unit, "min"):
		return amount * 60
	case strings.HasPrefix(unit, "hour") || strings.HasPrefix(unit, "hr"):
		return amount * 3600
	case strings.HasPrefix(unit, "day"):
		return amount * 86400
	case strings.HasPrefix(unit, "week"):
		return amount * 604800
	}
	return 0
}

// HandleNewLock inicia el flujo de creación de un nuevo lock
func (b *Bot) HandleNewLock(args string) {
	// Verificar que no haya sesión activa
	if _, err := b.chaster.GetActiveLock(); err == nil {
		b.Send("🔒 Ya tienes una sesión activa. Espera a que termine antes de crear una nueva.")
		return
	}

	// Parsear duración manual si fue proporcionada
	if args != "" {
		secs := parseDuration(args)
		if secs <= 0 {
			b.Send("❌ Invalid format. Examples:\n`/newlock 4 hours`\n`/newlock 1 minute`\n`/newlock 2 days`\n`/newlock 1 week`")
			return
		}
		b.state.ManualDurationSeconds = secs
	} else {
		b.state.ManualDurationSeconds = 0 // la IA decide
	}

	b.Send("▪️ *NUEVA SESIÓN*\n▬▬▬▬▬▬▬▬▬▬▬▬\nCierra el candado. Gira los diales sin mirar.\n\nCuando esté listo, manda la foto.\n▬▬▬▬▬▬▬▬▬▬▬▬\n_La imagen será eliminada automáticamente._")

	b.state.AwaitingLockPhoto = true
	b.saveState()
}

// HandleLockPhoto procesa la foto del candado para crear el lock
func (b *Bot) HandleLockPhoto(imageBytes []byte, mimeType string, messageID int) {
	// Borrar el mensaje con la foto inmediatamente
	b.deleteMessage(messageID)

	b.Send("_Verificando..._")

	// Verificar que el candado esté cerrado
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

	// Duración: manual o decidida por la IA
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

	// Subir foto a Chaster como combinación
	combinationID, err := b.chaster.UploadCombinationImage(imageBytes, mimeType)
	if err != nil {
		b.Send("❌ Error subiendo la combinación a Chaster.")
		return
	}

	// Crear el lock en modo test
	lockID, err := b.chaster.CreateLock(combinationID, durationSeconds, true)
	if err != nil {
		b.Send("❌ Error creando el lock en Chaster.")
		return
	}

	// Guardar lockID en estado para verificar cuando termine
	b.state.AwaitingLockPhoto = false
	b.state.CurrentLockID = lockID
	b.saveState()

	// Escapar caracteres especiales del mensaje de la IA para evitar errores de Markdown
	hours := durationSeconds / 3600
	mins := (durationSeconds % 3600) / 60
	var durStr string
	if hours > 0 {
		durStr = fmt.Sprintf("%dh", hours)
	} else {
		durStr = fmt.Sprintf("%dm", mins)
	}

	b.Send(fmt.Sprintf(
		"▪️ *SESIÓN INICIADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\n%s\n▬▬▬▬▬▬▬▬▬▬▬▬\nDuración — *%s*\nModo — test\n\n_Tu combinación está guardada. No la verás hasta que termine._",
		stripMarkdown(lockMsg),
		durStr,
	))
}

// ── Helpers adicionales ────────────────────────────────────────────────────

// deleteMessage borra un mensaje del chat
func (b *Bot) deleteMessage(messageID int) {
	del := tgbotapi.NewDeleteMessage(b.chatID, messageID)
	b.api.Request(del)
}

// CheckLockFinished verifica si el lock terminó y manda la imagen de combinación
func (b *Bot) CheckLockFinished() {
	if b.state.CurrentLockID == "" {
		return
	}

	// Verificar si el lock sigue activo
	lock, err := b.chaster.GetActiveLock()
	if err == nil && lock.ID == b.state.CurrentLockID {
		return
	}

	// Desbloquear el lock (ignorar error si ya está desbloqueado)
	b.chaster.UnlockLock(b.state.CurrentLockID)

	// Obtener combinación
	combo, err := b.chaster.GetCombination(b.state.CurrentLockID)
	if err != nil {
		log.Printf("error obteniendo combinación: %v", err)
		return
	}

	// Descargar imagen de combinación
	imgBytes, err := b.chaster.DownloadCombinationImage(combo.ImageFullURL)
	if err != nil {
		log.Printf("error descargando imagen de combinación: %v", err)
		b.Send("🔓 *SESIÓN TERMINADA*\n\nNo pude obtener la imagen de combinación. Revisa Chaster directamente.")
		return
	}

	// Mandar imagen por Telegram
	photoMsg := tgbotapi.NewPhoto(b.chatID, tgbotapi.FileBytes{
		Name:  "combinacion.jpg",
		Bytes: imgBytes,
	})
	photoMsg.Caption = "▪️ *SESIÓN TERMINADA*\n▬▬▬▬▬▬▬▬▬▬▬▬\nEsta es tu combinación.\nYa puedes liberarte."
	photoMsg.ParseMode = "Markdown"
	if _, err := b.api.Send(photoMsg); err != nil {
		log.Printf("error enviando foto de combinación: %v", err)
		b.Send("🔓 *SESIÓN TERMINADA* — revisa Chaster para ver tu combinación.")
	}

	// Archivar el lock
	if err := b.chaster.ArchiveLock(b.state.CurrentLockID); err != nil {
		log.Printf("error archivando lock: %v", err)
	}

	b.state.CurrentLockID = ""
	b.saveState()
}
