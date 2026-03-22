package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"chaster-keyholder/models"
	"chaster-keyholder/prompts"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

type Client struct {
	apiKey     string
	httpClient *http.Client
	P          *prompts.Loader
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// PhotoVerdict resultado de la validación de foto
type PhotoVerdict struct {
	Status string `json:"status"` // "approved", "retry", "rejected"
	Reason string `json:"reason"`
}

func NewClient(apiKey string, p *prompts.Loader) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		P:          p,
	}
}

// nonRetryableError is returned for HTTP 4xx errors (except 429) that should not be retried.
type nonRetryableError struct {
	StatusCode int
	Body       string
}

func (e *nonRetryableError) Error() string {
	return fmt.Sprintf("groq error %d: %s", e.StatusCode, e.Body)
}

// doRequest sends data to the Groq API with up to 3 attempts and exponential backoff.
// Retries on network errors, HTTP 429, and HTTP 5xx.
// Returns immediately on HTTP 4xx (except 429).
func (c *Client) doRequest(data []byte) ([]byte, error) {
	backoff := []time.Duration{1 * time.Second, 2 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		httpReq, err := http.NewRequest("POST", groqURL, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if attempt < 3 {
				log.Printf("[groq] retry %d/3: %v", attempt, err)
				time.Sleep(backoff[attempt-1])
			}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		// Non-retryable: 4xx except 429
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, &nonRetryableError{StatusCode: resp.StatusCode, Body: string(body)}
		}

		// Retryable: 429 or 5xx
		lastErr = fmt.Errorf("groq error %d: %s", resp.StatusCode, string(body))
		if attempt < 3 {
			log.Printf("[groq] retry %d/3: %v", attempt, lastErr)
			time.Sleep(backoff[attempt-1])
		}
	}
	return nil, lastErr
}

// ChatResult resultado del chat libre con detección de infracciones al contrato
type ChatResult struct {
	Message   string
	Violation *ChatViolation
}

// ChatViolation infracción detectada contra una regla del contrato activo
type ChatViolation struct {
	RuleID     string `json:"rule_id"`
	RuleText   string `json:"rule_text"`
	Punishment string `json:"punishment"` // "add_time" | "pillory" | "freeze"
	Hours      int    `json:"hours"`
	Minutes    int    `json:"minutes"`
}

// chatMessages envía una conversación completa (historial de mensajes) a la API de Groq.
// Se usa para el chat libre con Jolie, donde se pasa todo el historial para dar contexto.
// A diferencia de chat(), aquí el llamador controla los roles y el orden de los mensajes.
func (c *Client) chatMessages(model string, messages []Message, maxTokens int) (string, error) {
	req := ChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: 1.1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	body, err := c.doRequest(data)
	if err != nil {
		return "", err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("sin respuesta de la IA")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// chat envía un único intercambio system+user a la API de Groq.
// Se usa para todos los prompts "de una sola vuelta": generar tareas, mensajes,
// verificar fotos, etc. userContent puede ser string o []contentPart (para visión).
func (c *Client) chat(model, systemPrompt string, userContent interface{}) (string, error) {
	req := ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		MaxTokens:   600,
		Temperature: 1.1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	body, err := c.doRequest(data)
	if err != nil {
		return "", err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("sin respuesta de la IA")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// ── System prompt base ─────────────────────────────────────────────────────
// Los system prompts viven en prompts/prompts.yaml (system.locked / system.free).
// Se acceden via c.P.System.Locked, c.P.System.Free, o c.P.BuildSystemPrompt(locked).

// toyShortRef devuelve la referencia informal en español para un tipo de juguete.
// Se usa en prompts de personalidad para evitar que el modelo repita el nombre completo registrado.
// Los prompts de verificación de foto usan el nombre completo (necesario para identificar el objeto).
func toyShortRef(toyType string) string {
	switch toyType {
	case "cage":
		return "la jaula"
	case "plug":
		return "el plug"
	case "dildo":
		return "el dildo"
	case "vibrator":
		return "el vibrador"
	case "nipple":
		return "las pinzas"
	case "restraint":
		return "las ataduras"
	default:
		return "el juguete"
	}
}

// buildContext builds toy and intensity context for prompts
func buildContext(toys []models.Toy, daysLocked int) string {
	intensity := models.GetIntensity(daysLocked)

	inUse := []string{}
	available := []string{}
	for _, t := range toys {
		if t.InUse {
			inUse = append(inUse, toyShortRef(t.Type))
		} else {
			available = append(available, toyShortRef(t.Type))
		}
	}

	ctx := fmt.Sprintf("She has been locked for %d days. Intensity level: %s.", daysLocked, intensity.String())

	if len(inUse) > 0 {
		ctx += fmt.Sprintf(" Currently wearing: %s.", strings.Join(inUse, ", "))
	}
	if len(available) > 0 {
		ctx += fmt.Sprintf(" Available toys: %s.", strings.Join(available, ", "))
	}
	if len(inUse) == 0 && len(available) == 0 {
		ctx += " No toys registered."
	}

	return ctx
}

// buildContextFree context when there is no active session
func buildContextFree(toys []models.Toy) string {
	toyRefs := []string{}
	for _, t := range toys {
		toyRefs = append(toyRefs, toyShortRef(t.Type))
	}
	toyContext := "no toys registered"
	if len(toyRefs) > 0 {
		toyContext = strings.Join(toyRefs, ", ")
	}
	return fmt.Sprintf("She is currently free. Available toys: %s.", toyContext)
}

// ── Automatic messages ─────────────────────────────────────────────────────

func (c *Client) GenerateMorningMessage(daysLocked int, timeRemaining string, toys []models.Toy, daysSinceLastOrgasm int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	orgasmCtx := ""
	if daysSinceLastOrgasm > 7 {
		orgasmCtx = fmt.Sprintf(" She has not orgasmed in %d days — reference this subtly as part of her sentence.", daysSinceLastOrgasm)
	} else if daysSinceLastOrgasm < 0 {
		orgasmCtx = " She has never orgasmed — the cage has never let her."
	}
	prompt := c.P.MustRender("morning_message", map[string]any{
		"Ctx":           ctx,
		"TimeRemaining": timeRemaining,
		"OrgasmCtx":     orgasmCtx,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

func (c *Client) GenerateNightMessage(daysLocked int, taskCompleted bool, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	status := "completed her task today"
	if !taskCompleted {
		status = "did NOT complete her task and was penalized — she disappointed Papi"
	}
	prompt := c.P.MustRender("night_message", map[string]any{
		"Ctx":    ctx,
		"Status": status,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Tasks ──────────────────────────────────────────────────────────────────

func (c *Client) GenerateDailyTask(daysLocked int, toys []models.Toy, level models.IntensityLevel, recentTasks []string) (string, error) {
	ctx := buildContext(toys, daysLocked)

	recentCtx := ""
	if len(recentTasks) > 0 {
		recentCtx = "\n\nRECENT TASKS — do NOT repeat these or anything similar:\n"
		for i, t := range recentTasks {
			recentCtx += fmt.Sprintf("%d. %s\n", i+1, t)
		}
	}

	prompt := c.P.MustRender("daily_task", map[string]any{
		"Ctx":       ctx,
		"Level":     level.String(),
		"RecentCtx": recentCtx,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// GenerateTaskExplanation explains in detail how to take the photo for the current task
func (c *Client) GenerateTaskExplanation(taskDescription string, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := c.P.MustRender("task_explanation", map[string]any{
		"Ctx":             ctx,
		"TaskDescription": taskDescription,
	})
	return c.chat(c.P.Models.Text, c.P.System.TaskExplanation, prompt)
}

// GenerateTaskAccepted generates a lewd, possessive reaction to seeing Jolie's photo.
func (c *Client) GenerateTaskAccepted(toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := c.P.MustRender("task_accepted", map[string]any{"Ctx": ctx})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// GenerateTaskReward generates a reward message. rewardHours in HOURS.
func (c *Client) GenerateTaskReward(rewardHours int, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := c.P.MustRender("task_reward", map[string]any{
		"Ctx":         ctx,
		"RewardHours": rewardHours,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// GenerateTaskPenalty generates a penalty message. penaltyHours in HOURS.
func (c *Client) GenerateTaskPenalty(penaltyHours int, reason string) (string, error) {
	prompt := c.P.MustRender("task_penalty", map[string]any{
		"Reason":       reason,
		"PenaltyHours": penaltyHours,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Photo verification with Vision ─────────────────────────────────────────

func (c *Client) VerifyTaskPhoto(imageBytes []byte, mimeType, taskDescription string, toys []models.Toy, daysLocked int) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	ctx := buildContext(toys, daysLocked)
	textPrompt := c.P.MustRender("verify_task_photo_user", map[string]any{
		"TaskDescription": taskDescription,
		"Ctx":             ctx,
	})

	userContent := []contentPart{
		{Type: "text", Text: textPrompt},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, c.P.System.VerifyTaskPhoto, userContent)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var verdict PhotoVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return &PhotoVerdict{Status: "rejected", Reason: raw}, nil
	}
	if verdict.Status == "" {
		verdict.Status = "rejected"
	}
	return &verdict, nil
}

// ── New lock ───────────────────────────────────────────────────────────────

// LockDecision AI decision on lock duration
type LockDecision struct {
	DurationHours int    `json:"duration_hours"`
	Message       string `json:"message"`
}

// DecideLockDuration the AI decides how long the lock should last
func (c *Client) DecideLockDuration(daysHistory int, toys []models.Toy) (*LockDecision, error) {
	ctx := buildContext(toys, daysHistory)
	system := c.P.System.Free + "\n" + c.P.Get("lock_duration")
	prompt := c.P.MustRender("decide_lock_duration", map[string]any{"Ctx": ctx})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var decision LockDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &LockDecision{DurationHours: c.P.Thresholds.LockDurationDefault, Message: "12 horas bajo mi control."}, nil
	}
	if decision.DurationHours <= 0 {
		decision.DurationHours = c.P.Thresholds.LockDurationDefault
	}
	return &decision, nil
}

// GenerateContract genera las reglas del contrato de sesión que Papi impone.
func (c *Client) GenerateContract(lockDurationHours int, toys []models.Toy, daysHistory int) (string, error) {
	daysStr := ""
	if lockDurationHours >= 24 {
		d := lockDurationHours / 24
		daysStr = fmt.Sprintf("%d día(s)", d)
	} else {
		daysStr = fmt.Sprintf("%d hora(s)", lockDurationHours)
	}

	ctx := buildContext(toys, daysHistory)
	prompt := c.P.MustRender("generate_contract", map[string]any{
		"DaysStr": daysStr,
		"Ctx":     ctx,
	})

	resp, err := c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp), nil
}

// extractJSON extrae el primer bloque JSON válido de una respuesta de la IA.
// Necesario porque los modelos de lenguaje a veces envuelven el JSON en bloques
// de código markdown (```json ... ```) o añaden texto antes/después del objeto.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// VerifyLockPhoto verifies that the photo shows the closed lock with visible combination
func (c *Client) VerifyLockPhoto(imageBytes []byte, mimeType string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	userContent := []contentPart{
		{Type: "text", Text: "Does this photo show BOTH a Kingsley combination lock box AND a chastity cage worn on the body? Be generous — approve if both elements are reasonably visible."},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, c.P.System.VerifyLockPhoto, userContent)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var verdict PhotoVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return &PhotoVerdict{Status: "rejected", Reason: raw}, nil
	}
	if verdict.Status == "" {
		verdict.Status = "rejected"
	}
	return &verdict, nil
}

// ── Clasificador de intención ──────────────────────────────────────────────

// IntentResult resultado del clasificador de intención del chat libre.
type IntentResult struct {
	Intent string `json:"intent"` // "lock_request"|"toy_request"|"cum_request"|"cum_report"|"toy_confirm"|"chat"
	Toy    string `json:"toy,omitempty"`
}

// ClassifyIntent clasifica un mensaje en lenguaje natural en una intención de acción.
// Es una llamada ligera y rápida — el modelo solo devuelve JSON corto.
func (c *Client) ClassifyIntent(message string) (*IntentResult, error) {
	userPrompt := c.P.MustRender("classify_intent_user", map[string]any{"Message": message})
	raw, err := c.chat(c.P.Models.Text, c.P.System.ClassifyIntent, userPrompt)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var result IntentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return &IntentResult{Intent: "chat"}, nil
	}
	return &result, nil
}

// ── Sesión de juguetes ─────────────────────────────────────────────────────

// GenerateToySessionGranted genera el mensaje de Papi al conceder una sesión de juguetes.
// Devuelve message (reacción) + condition (instrucciones específicas de cómo usarlos).
func (c *Client) GenerateToySessionGranted(toys []models.Toy, daysLocked int) (*OrgasmDecision, error) {
	ctx := buildContext(toys, daysLocked)

	var toyNames []string
	for _, t := range toys {
		if t.Type != "cage" {
			toyNames = append(toyNames, toyShortRef(t.Type))
		}
	}
	toyList := "ninguno registrado"
	if len(toyNames) > 0 {
		toyList = strings.Join(toyNames, ", ")
	}

	suffix := c.P.MustRender("toy_session_granted", map[string]any{"ToyList": toyList})
	system := c.P.System.Locked + "\n" + suffix

	raw, err := c.chat(c.P.Models.Text, system, ctx)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var decision OrgasmDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &OrgasmDecision{Outcome: "granted_toys", Message: raw}, nil
	}
	decision.Outcome = "granted_toys"
	return &decision, nil
}

// GenerateToySessionDenied genera el mensaje de Papi al negar una sesión de juguetes.
// reason: "debt" (deuda alta) | "cooldown" (sesión reciente)
func (c *Client) GenerateToySessionDenied(reason string, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	suffixKey := "toy_denied_cooldown"
	if reason == "debt" {
		suffixKey = "toy_denied_debt"
	}
	system := c.P.System.Locked + "\n" + c.P.Get(suffixKey)
	return c.chat(c.P.Models.Text, system, ctx)
}

// ── Insistencia durante cooldown ───────────────────────────────────────────

// GenerateInsistenceResponse genera la reacción de Papi cuando Jolie insiste durante cooldown.
// attempt: 1 (primera insistencia) o 2 (segunda — la siguiente es el roll especial).
func (c *Client) GenerateInsistenceResponse(attempt int, hoursLeft float64) (string, error) {
	suffixKey := "insistence_attempt_2"
	if attempt == 1 {
		suffixKey = "insistence_attempt_1"
	}
	suffix := c.P.MustRender(suffixKey, map[string]any{"HoursLeft": hoursLeft})
	system := c.P.System.Locked + "\n" + suffix
	return c.chat(c.P.Models.Text, system, "She begs again")
}

// GenerateInsistenceRollMessage genera la reacción de Papi al resultado del roll de insistencia.
// outcome: "granted_cum" | "granted_toys" | "punished"
// punishHours: horas añadidas al candado si punished (para que Papi las mencione).
func (c *Client) GenerateInsistenceRollMessage(outcome string, toys []models.Toy, daysLocked, streak, punishHours int) (*OrgasmDecision, error) {
	ctx := buildContext(toys, daysLocked)

	var toyNames []string
	for _, t := range toys {
		if t.Type != "cage" {
			toyNames = append(toyNames, toyShortRef(t.Type))
		}
	}
	toyList := "ninguno"
	if len(toyNames) > 0 {
		toyList = strings.Join(toyNames, ", ")
	}

	var suffixKey string
	var data map[string]any
	switch outcome {
	case "granted_cum":
		suffixKey = "insistence_roll_granted_cum"
		data = map[string]any{"ToyList": toyList}
	case "granted_toys":
		suffixKey = "insistence_roll_granted_toys"
		data = map[string]any{"ToyList": toyList}
	default: // punished
		suffixKey = "insistence_roll_punished"
		data = map[string]any{"PunishHours": punishHours}
	}

	suffix := c.P.MustRender(suffixKey, data)
	system := c.P.System.Locked + "\n" + suffix
	raw, err := c.chat(c.P.Models.Text, system, fmt.Sprintf("%s\nStreak: %d", ctx, streak))
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var decision OrgasmDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &OrgasmDecision{Outcome: outcome, Message: raw}, nil
	}
	decision.Outcome = outcome
	return &decision, nil
}

// ── Permiso de orgasmo ─────────────────────────────────────────────────────

// OrgasmDecision resultado de una solicitud de permiso de orgasmo
type OrgasmDecision struct {
	Outcome   string `json:"outcome"`             // "denied" | "granted_cum" | "granted_toys" | "punished"
	Message   string `json:"message"`
	Condition string `json:"condition,omitempty"` // instrucciones si granted
}

// GenerateOrgasmMessage genera el texto de Papi para un outcome ya decidido.
// outcome: "denied" | "granted_cum"
func (c *Client) GenerateOrgasmMessage(outcome, userMessage string, toys []models.Toy, daysLocked, streak, daysSinceLastOrgasm, consecutiveDenials int) (*OrgasmDecision, error) {
	ctx := buildContext(toys, daysLocked)

	lastOrgasmStr := "NEVER — she has never orgasmed as Papi's sissy. Do NOT invent a number of days."
	if daysSinceLastOrgasm >= 0 && daysSinceLastOrgasm < 999 {
		if daysSinceLastOrgasm == 0 {
			lastOrgasmStr = "today"
		} else {
			lastOrgasmStr = fmt.Sprintf("%d days ago", daysSinceLastOrgasm)
		}
	}

	var toyNames []string
	for _, t := range toys {
		if t.Type != "cage" {
			toyNames = append(toyNames, toyShortRef(t.Type))
		}
	}
	toyList := "none"
	if len(toyNames) > 0 {
		toyList = strings.Join(toyNames, ", ")
	}

	// La instrucción de outcome se selecciona en Go; el texto va en YAML
	var outcomeInstruction string
	switch outcome {
	case "granted_cum":
		outcomeInstruction = c.P.Get("orgasm_granted_instruction")
	default: // "denied"
		var deniedOrgasmRef string
		if daysSinceLastOrgasm >= 999 {
			deniedOrgasmRef = `She has NEVER orgasmed — reference this: "nunca te has corrido" or "no sabes ni lo que es correrte".`
		} else {
			deniedOrgasmRef = fmt.Sprintf(`Reference her wait: "llevas %d días sin correrte" or "te lo di hace %d días".`, daysSinceLastOrgasm, daysSinceLastOrgasm)
		}
		outcomeInstruction = c.P.MustRender("orgasm_denied_instruction", map[string]any{
			"DeniedOrgasmRef": deniedOrgasmRef,
		})
	}

	consecutiveLine := ""
	if consecutiveDenials >= 3 {
		consecutiveLine = fmt.Sprintf("\nConsecutive denials so far: %d — she's getting desperate.", consecutiveDenials)
	}

	preamble := c.P.MustRender("orgasm_system_preamble", map[string]any{"ToyList": toyList})
	system := c.P.System.Locked + "\n" + preamble + "\n" + outcomeInstruction + "\n" + c.P.Get("orgasm_system_rules")

	prompt := c.P.MustRender("orgasm_message", map[string]any{
		"Ctx":            ctx,
		"Streak":         streak,
		"LastOrgasmStr":  lastOrgasmStr,
		"ConsecutiveLine": consecutiveLine,
		"UserMessage":    userMessage,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var decision OrgasmDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &OrgasmDecision{Outcome: outcome, Message: raw}, nil
	}
	decision.Outcome = outcome // enforce the pre-rolled outcome
	return &decision, nil
}

// GenerateOrgasmCooldownMessage genera un mensaje en personaje cuando se pide permiso demasiado pronto.
// lastOutcome: "denied" | "granted_cum" | "granted_toys" — para que Papi haga referencia a lo anterior.
// hoursLeft: horas restantes del cooldown.
func (c *Client) GenerateOrgasmCooldownMessage(lastOutcome string, hoursLeft float64) (string, error) {
	var suffixKey string
	switch lastOutcome {
	case "granted_cum", "granted":
		suffixKey = "orgasm_cooldown_granted"
	case "granted_toys":
		suffixKey = "orgasm_cooldown_granted_toys"
	default:
		suffixKey = "orgasm_cooldown_denied"
	}
	system := c.P.System.Locked + "\n" + c.P.MustRender(suffixKey, map[string]any{"HoursLeft": hoursLeft})
	return c.chat(c.P.Models.Text, system, "She asks: permission please")
}

// GenerateCameResponse genera la reacción de Papi cuando Jolie reporta que se vino.
// permitted=true si había permiso activo, false si fue una violación.
// daysSinceLastOrgasm: días desde el orgasmo anterior (-1 si nunca).
// grantedCondition: instrucciones específicas que Papi dio al conceder el permiso (puede ser "").
func (c *Client) GenerateCameResponse(method, toyName string, permitted bool, daysLocked, daysSinceLastOrgasm int, grantedCondition string, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)

	methodDesc := method
	if toyName != "" {
		methodDesc = fmt.Sprintf("%s (%s)", method, toyName)
	}

	waitLine := ""
	switch {
	case daysSinceLastOrgasm < 0:
		waitLine = "This is her FIRST orgasm ever as Papi's sissy — reference this: never came before."
	case daysSinceLastOrgasm == 0:
		waitLine = "She came earlier today."
	default:
		waitLine = fmt.Sprintf("She waited %d days since her last orgasm.", daysSinceLastOrgasm)
	}

	var suffixKey string
	var data map[string]any
	if permitted {
		conditionRef := ""
		if strings.TrimSpace(grantedCondition) != "" {
			conditionRef = fmt.Sprintf("\nPapi's original order was: \"%s\" — reference that she followed it specifically.", grantedCondition)
		}
		suffixKey = "came_permitted"
		data = map[string]any{
			"MethodDesc":   methodDesc,
			"WaitLine":     waitLine,
			"ConditionRef": conditionRef,
		}
	} else {
		suffixKey = "came_not_permitted"
		data = map[string]any{
			"MethodDesc": methodDesc,
			"WaitLine":   waitLine,
		}
	}

	system := c.P.System.Locked + "\n" + c.P.MustRender(suffixKey, data)
	return c.chat(c.P.Models.Text, system, ctx)
}

// ── Free chat ──────────────────────────────────────────────────────────────

// NegotiationResult result of a time negotiation with the keyholder
type NegotiationResult struct {
	Decision    string `json:"decision"`   // "approved", "rejected", "counter", "penalty"
	TimeHours   int    `json:"time_hours"` // positive = add, negative = remove
	Message     string `json:"message"`
	CounterTask string `json:"counter_task,omitempty"`
}

// Chat free conversation with the keyholder. totalHoursAdded in HOURS.
// locked indicates if there is an active session — changes the system prompt.
// history: recent messages (user/assistant pairs). rules: active contract rules to enforce.
func (c *Client) Chat(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int, locked bool, history []models.ChatMessage, rules []models.ContractRule) (*ChatResult, error) {
	system := c.P.BuildSystemPrompt(locked)

	var userPrompt string
	if locked {
		ctx := buildContext(toys, daysLocked)
		userPrompt = c.P.MustRender("chat_locked", map[string]any{
			"Ctx":             ctx,
			"TasksCompleted":  tasksCompleted,
			"TasksFailed":     tasksFailed,
			"TotalHoursAdded": totalHoursAdded,
			"UserMessage":     userMessage,
		})
	} else {
		ctx := buildContextFree(toys)
		userPrompt = c.P.MustRender("chat_free", map[string]any{
			"Ctx":         ctx,
			"UserMessage": userMessage,
		})
	}

	hasRules := locked && len(rules) > 0
	if hasRules {
		rulesText := "\n\nACTIVE CONTRACT RULES — you enforce these automatically:\n"
		for i, r := range rules {
			penalty := r.Punishment
			if r.Hours > 0 {
				penalty += fmt.Sprintf(" +%dh", r.Hours)
			} else if r.Minutes > 0 {
				penalty += fmt.Sprintf(" %dmin", r.Minutes)
			}
			rulesText += fmt.Sprintf("%d. [%s] %s → %s\n", i+1, r.ID, r.RuleText, penalty)
		}
		rulesText += `
After writing your response, evaluate if Jolie's message clearly violates any of the above rules.
Only flag CLEAR, UNAMBIGUOUS violations — when in doubt, do NOT flag.
Respond ONLY in valid JSON:
{"message": "your response in Spanish as Papi", "violation": {"rule_id": "...", "rule_text": "...", "punishment": "add_time|pillory|freeze", "hours": N, "minutes": N}}
or if no violation:
{"message": "your response in Spanish as Papi", "violation": null}`
		system += rulesText
	}

	messages := []Message{{Role: "system", Content: system}}
	for _, h := range history {
		messages = append(messages, Message{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, Message{Role: "user", Content: userPrompt})

	maxTokens := 600
	if hasRules {
		maxTokens = 750
	}

	raw, err := c.chatMessages(c.P.Models.Text, messages, maxTokens)
	if err != nil {
		return nil, err
	}

	if hasRules {
		clean := extractJSON(raw)
		var parsed struct {
			Message   string         `json:"message"`
			Violation *ChatViolation `json:"violation"`
		}
		if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
			return &ChatResult{Message: raw}, nil
		}
		return &ChatResult{Message: parsed.Message, Violation: parsed.Violation}, nil
	}

	return &ChatResult{Message: raw}, nil
}

// ExtractContractRules analiza el texto del contrato y extrae las reglas verificables por chat.
func (c *Client) ExtractContractRules(contractText, lockID string) ([]models.ContractRule, error) {
	prompt := fmt.Sprintf("Contract:\n%s\n\nExtract only chat-verifiable rules as JSON array.", contractText)
	raw, err := c.chat(c.P.Models.Text, c.P.System.ExtractContractRules, prompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON array
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, nil // no verifiable rules
	}
	raw = raw[start : end+1]

	type ruleJSON struct {
		ID         string `json:"id"`
		RuleText   string `json:"rule_text"`
		Punishment string `json:"punishment"`
		Hours      int    `json:"hours"`
		Minutes    int    `json:"minutes"`
	}
	var parsed []ruleJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}

	rules := make([]models.ContractRule, 0, len(parsed))
	for _, r := range parsed {
		if r.ID == "" || r.RuleText == "" || r.Punishment == "" {
			continue
		}
		rules = append(rules, models.ContractRule{
			ID:         r.ID,
			LockID:     lockID,
			RuleText:   r.RuleText,
			Punishment: r.Punishment,
			Hours:      r.Hours,
			Minutes:    r.Minutes,
		})
	}
	return rules, nil
}

// NegotiateTime evaluates a time negotiation request. totalHoursAdded in HOURS.
func (c *Client) NegotiateTime(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (*NegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)
	system := c.P.System.Locked + "\n" + c.P.Get("negotiate_time")
	prompt := c.P.MustRender("negotiate_time", map[string]any{
		"Ctx":             ctx,
		"TasksCompleted":  tasksCompleted,
		"TasksFailed":     tasksFailed,
		"TotalHoursAdded": totalHoursAdded,
		"UserMessage":     userMessage,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var result NegotiationResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return &NegotiationResult{
			Decision:  "rejected",
			TimeHours: 0,
			Message:   raw,
		}, nil
	}
	return &result, nil
}

// ── Random events ──────────────────────────────────────────────────────────

// RandomEventDecision AI decision on which random event to execute
type RandomEventDecision struct {
	Action          string `json:"action"`           // "freeze" | "hidetime" | "pillory" | "addtime" | "none"
	DurationMinutes int    `json:"duration_minutes"` // event duration
	Message         string `json:"message"`          // dominant message
	Reason          string `json:"reason"`           // internal reason (for logs)
}

// DecideRandomEvent the AI decides which random event to execute based on context
func (c *Client) DecideRandomEvent(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hourOfDay int, hasActiveEvent bool) (*RandomEventDecision, error) {
	ctx := buildContext(toys, daysLocked)
	system := c.P.System.Locked + "\n" + c.P.Get("random_event")
	prompt := c.P.MustRender("random_event", map[string]any{
		"Ctx":            ctx,
		"TasksCompleted": tasksCompleted,
		"TasksFailed":    tasksFailed,
		"Hour":           hourOfDay,
		"HasActiveEvent": hasActiveEvent,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var decision RandomEventDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &RandomEventDecision{Action: "none", Reason: "error parsing response"}, nil
	}

	// Safety validations — mínimos definidos en prompts.yaml [thresholds]
	if decision.Action == "pillory" && decision.DurationMinutes < c.P.Thresholds.PilloryMinMinutes {
		decision.DurationMinutes = c.P.Thresholds.PilloryMinMinutes
	}
	if decision.Action == "freeze" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = c.P.Thresholds.FreezeDefaultMinutes
	}
	if decision.Action == "hidetime" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = c.P.Thresholds.HidetimeDefaultMinutes
	}
	if decision.Action == "addtime" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = c.P.Thresholds.AddtimeDefaultMinutes
	}

	return &decision, nil
}

// NegotiateActiveEvent evaluates a plea to revert an active event
type EventNegotiationResult struct {
	Decision string `json:"decision"` // "approved" | "rejected" | "counter" | "penalty"
	Message  string `json:"message"`
	Task     string `json:"task,omitempty"` // if asking for something in return
}

func (c *Client) NegotiateActiveEvent(userMessage string, eventType string, minutesRemaining int, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int) (*EventNegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	eventDesc := map[string]string{
		"freeze":   "lock freeze",
		"hidetime": "hidden timer",
	}[eventType]
	if eventDesc == "" {
		eventDesc = eventType
	}

	system := c.P.System.Locked + "\n" + c.P.Get("active_event")
	prompt := c.P.MustRender("active_event", map[string]any{
		"Ctx":              ctx,
		"TasksCompleted":   tasksCompleted,
		"TasksFailed":      tasksFailed,
		"EventDesc":        eventDesc,
		"MinutesRemaining": minutesRemaining,
		"UserMessage":      userMessage,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var result EventNegotiationResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return &EventNegotiationResult{Decision: "rejected", Message: raw}, nil
	}
	return &result, nil
}

// ── Random control messages ────────────────────────────────────────────────

// GenerateRandomMessage generates a spontaneous keyholder message with no task context.
// messageType forces a specific style — pass empty string to let AI decide.
// locked indicates if there is an active session.
func (c *Client) GenerateRandomMessage(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hasActiveEvent bool, activeEventType string, locked bool, messageType string, todayContext string, daysSinceLastOrgasm int) (string, error) {
	system := c.P.BuildSystemPrompt(locked)

	if !locked {
		ctx := buildContextFree(toys)
		prompt := c.P.MustRender("random_message_free", map[string]any{"Ctx": ctx})
		return c.chat(c.P.Models.Text, system, prompt)
	}

	ctx := buildContext(toys, daysLocked)

	eventCtx := ""
	if hasActiveEvent {
		switch activeEventType {
		case "freeze":
			eventCtx = "Her lock is currently frozen — she is immobilized."
		case "hidetime":
			eventCtx = "She cannot see her timer right now — she doesn't know how long she has left."
		}
	}

	typeInstruction := messageType
	if typeInstruction == "" {
		styles := c.P.Lists["random_message_styles"]
		typeInstruction = styles[rand.Intn(len(styles))]
	}

	extraCtx := ""
	if todayContext != "" {
		extraCtx += fmt.Sprintf(" Today: %s.", todayContext)
	}
	if daysSinceLastOrgasm > 5 {
		extraCtx += fmt.Sprintf(" She has not orgasmed in %d days.", daysSinceLastOrgasm)
	} else if daysSinceLastOrgasm < 0 {
		extraCtx += " She has never orgasmed."
	}

	prompt := c.P.MustRender("random_message_locked", map[string]any{
		"Ctx":             ctx,
		"TasksCompleted":  tasksCompleted,
		"TasksFailed":     tasksFailed,
		"EventCtx":        eventCtx,
		"ExtraCtx":        extraCtx,
		"TypeInstruction": typeInstruction,
	})

	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// GeneratePilloryReason generates a pillory reason in English (for the Chaster community)
func (c *Client) GeneratePilloryReason(daysLocked int, toys []models.Toy, context string) (string, error) {
	prompt := c.P.MustRender("pillory_reason", map[string]any{
		"DaysLocked": daysLocked,
		"Context":    context,
	})
	return c.chat(c.P.Models.Text, c.P.System.PilloryReason, prompt)
}

// ── Toys ───────────────────────────────────────────────────────────────────

// ToyInfo name, description and type generated by AI for a toy
type ToyInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "cage", "plug", "vibrator", "restraint", "other"
}

// DescribeToy analyzes a toy photo and generates name, description and type
func (c *Client) DescribeToy(imageBytes []byte, mimeType, hint string) (*ToyInfo, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	prompt := c.P.MustRender("describe_toy", map[string]any{"Hint": hint})

	userContent := []contentPart{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, c.P.System.DescribeToy, userContent)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)
	var info ToyInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return &ToyInfo{Name: hint, Description: "Juguete registrado."}, nil
	}
	if info.Name == "" {
		info.Name = hint
	}
	return &info, nil
}

// ── Clothing ───────────────────────────────────────────────────────────────

// ClothingInfo generated by vision AI from a clothing photo
type ClothingInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "thong"|"bra"|"stockings"|"socks"|"collar"|"lingerie"|"dress"|"top"|"bottom"|"shoes"|"accessory"|"other"
}

// DescribeClothing analyzes a clothing photo and returns name, description, type
func (c *Client) DescribeClothing(imageBytes []byte, mimeType string) (*ClothingInfo, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	userContent := []contentPart{
		{Type: "text", Text: c.P.Get("describe_clothing")},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, c.P.System.DescribeClothing, userContent)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var info ClothingInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return &ClothingInfo{Name: "Prenda", Description: "Prenda registrada.", Type: "other"}, nil
	}
	if info.Name == "" {
		info.Name = "Prenda"
	}
	return &info, nil
}

// OutfitAssignment contains the daily outfit message, description for verification, and required pose
type OutfitAssignment struct {
	Message     string `json:"message"`
	Description string `json:"description"`
	Pose        string `json:"pose"` // English description of the pose required in the photo
}

// GenerateOutfitAssignment selects items from the wardrobe and generates a dominant outfit assignment
func (c *Client) GenerateOutfitAssignment(daysLocked int, wardrobeItems []string, intensity models.IntensityLevel) (*OutfitAssignment, error) {
	ctx := fmt.Sprintf("She has been locked for %d days. Intensity: %s.", daysLocked, intensity.String())
	itemList := strings.Join(wardrobeItems, "\n- ")
	prompt := c.P.MustRender("outfit_assignment", map[string]any{
		"Ctx":      ctx,
		"ItemList": itemList,
	})

	raw, err := c.chat(c.P.Models.Text, c.P.System.Locked, []contentPart{{Type: "text", Text: prompt}})
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var out OutfitAssignment
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("error parsing outfit assignment JSON: %w — raw: %s", err, raw)
	}
	return &out, nil
}

// VerifyOutfitPhoto checks that the submitted photo matches the assigned outfit and pose
func (c *Client) VerifyOutfitPhoto(imageBytes []byte, mimeType, outfitDescription, poseDescription string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := c.P.MustRender("verify_outfit_photo_system", map[string]any{
		"OutfitDescription": outfitDescription,
		"PoseDescription":   poseDescription,
	})

	userContent := []contentPart{
		{Type: "text", Text: "Verify the outfit and pose in this photo."},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, system, userContent)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var v PhotoVerdict
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return &PhotoVerdict{Status: "approved", Reason: "foto aceptada"}, nil
	}
	return &v, nil
}

// GenerateOutfitComment generates a dominant approval comment after a photo is accepted
func (c *Client) GenerateOutfitComment(daysLocked int, outfitDescription, poseDescription string) (string, error) {
	prompt := c.P.MustRender("outfit_comment", map[string]any{
		"DaysLocked":        daysLocked,
		"OutfitDescription": outfitDescription,
		"PoseDescription":   poseDescription,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, []contentPart{{Type: "text", Text: prompt}})
}

// ── Obediencia ─────────────────────────────────────────────────────────────
// Los strings por nivel viven en prompts.yaml [obedience_context].
// Se acceden via c.P.ObedienceCtx(level) en cada función que los necesita.

// ── Ritual matutino ────────────────────────────────────────────────────────

// GenerateRitualIntro sends the morning ritual instruction (step 1: photo)
func (c *Client) GenerateRitualIntro(daysLocked int, toys []models.Toy, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := c.P.MustRender("ritual_intro", map[string]any{
		"Ctx":          ctx,
		"ObedienceCtx": c.P.ObedienceCtx(obedienceLevel),
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// GenerateRitualResponse responds after ritual is complete and grants permission to work
func (c *Client) GenerateRitualResponse(userMessage string, daysLocked int, toys []models.Toy, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := c.P.MustRender("ritual_response", map[string]any{
		"Ctx":          ctx,
		"ObedienceCtx": c.P.ObedienceCtx(obedienceLevel),
		"UserMessage":  userMessage,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Plug diario ────────────────────────────────────────────────────────────

// GeneratePlugAssignment generates the plug assignment message for the day
func (c *Client) GeneratePlugAssignment(plugName string, daysLocked int, obedienceLevel int) (string, error) {
	prompt := c.P.MustRender("plug_assignment", map[string]any{
		"DaysLocked":   daysLocked,
		"ObedienceCtx": c.P.ObedienceCtx(obedienceLevel),
		"PlugName":     toyShortRef("plug"),
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// VerifyPlugPhoto verifies that the photo shows the assigned plug in use
func (c *Client) VerifyPlugPhoto(imageBytes []byte, mimeType, plugName string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	data := map[string]any{"PlugName": plugName}
	system := c.P.MustRender("verify_plug_photo_system", data)
	userText := c.P.MustRender("verify_plug_photo_user", data)

	userContent := []contentPart{
		{Type: "text", Text: userText},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, system, userContent)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var verdict PhotoVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return &PhotoVerdict{Status: "rejected", Reason: raw}, nil
	}
	if verdict.Status == "" {
		verdict.Status = "rejected"
	}
	return &verdict, nil
}

// ── Check-ins espontáneos ──────────────────────────────────────────────────

// GenerateCheckinRequest generates a sudden check-in demand
func (c *Client) GenerateCheckinRequest(daysLocked int, assignedPlugName string) (string, error) {
	plugInfo := ""
	if assignedPlugName != "" {
		plugInfo = " Tu plug también debe ser visible en la foto."
	}
	prompt := c.P.MustRender("checkin_request", map[string]any{
		"DaysLocked": daysLocked,
		"PlugInfo":   plugInfo,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// VerifyCheckinPhoto verifies the check-in photo shows cage (and plug if assigned)
func (c *Client) VerifyCheckinPhoto(imageBytes []byte, mimeType, assignedPlugName string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	plugReq := ""
	if assignedPlugName != "" {
		plugReq = fmt.Sprintf("\n2. The %s must also be visible and clearly in use.", assignedPlugName)
	}

	system := c.P.MustRender("verify_checkin_photo_system", map[string]any{"PlugReq": plugReq})

	userContent := []contentPart{
		{Type: "text", Text: "Does this check-in photo meet the requirements?"},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat(c.P.Models.Vision, system, userContent)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var verdict PhotoVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return &PhotoVerdict{Status: "rejected", Reason: raw}, nil
	}
	if verdict.Status == "" {
		verdict.Status = "rejected"
	}
	return &verdict, nil
}

// ── Condicionamiento ───────────────────────────────────────────────────────

// GenerateConditioningMessage generates a spontaneous conditioning phrase during work hours
func (c *Client) GenerateConditioningMessage(daysLocked int, toys []models.Toy, hour, obedienceLevel int, todayContext string, daysSinceLastOrgasm int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	extraCtx := ""
	if todayContext != "" {
		extraCtx += fmt.Sprintf(" Today: %s.", todayContext)
	}
	if daysSinceLastOrgasm > 5 {
		extraCtx += fmt.Sprintf(" She has not orgasmed in %d days.", daysSinceLastOrgasm)
	} else if daysSinceLastOrgasm < 0 {
		extraCtx += " She has never orgasmed."
	}
	prompt := c.P.MustRender("conditioning_message", map[string]any{
		"Ctx":          ctx,
		"ObedienceCtx": c.P.ObedienceCtx(obedienceLevel),
		"Hour":         hour,
		"ExtraCtx":     extraCtx,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Ruleta ─────────────────────────────────────────────────────────────────

// RuletaOutcome the result of a roulette spin
type RuletaOutcome struct {
	Action  string `json:"action"`  // "remove_time"|"add_time"|"pillory"|"freeze"|"hide_time"|"extra_task"|"reward"
	Value   int    `json:"value"`   // hours for time, minutes for events
	Message string `json:"message"` // dominant message in Spanish
}

// SpinRuleta lets the AI decide a random roulette outcome
func (c *Client) SpinRuleta(daysLocked int, toys []models.Toy, tasksCompleted, tasksFailed, obedienceLevel int) (*RuletaOutcome, error) {
	ctx := buildContext(toys, daysLocked)
	system := c.P.System.Locked + "\n" + c.P.Get("spin_ruleta")
	prompt := c.P.MustRender("spin_ruleta", map[string]any{
		"Ctx":            ctx,
		"ObedienceCtx":   c.P.ObedienceCtx(obedienceLevel),
		"TasksCompleted": tasksCompleted,
		"TasksFailed":    tasksFailed,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var outcome RuletaOutcome
	if err := json.Unmarshal([]byte(raw), &outcome); err != nil {
		return &RuletaOutcome{Action: "add_time", Value: 1, Message: "La ruleta ha hablado. +1h."}, nil
	}
	if outcome.Value < 0 {
		outcome.Value = -outcome.Value
	}
	return &outcome, nil
}

// ── Streak rewards ─────────────────────────────────────────────────────────

// GenerateStreakReward generates a message when Jolie earns a new obedience title.
func (c *Client) GenerateStreakReward(points int, daysLocked int, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	title := models.ObedienceTitle(points)
	prompt := c.P.MustRender("streak_reward", map[string]any{
		"Ctx":    ctx,
		"Title":  title,
		"Points": points,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Estado de ánimo ────────────────────────────────────────────────────────

// GenerateMoodMessage genera un mensaje de Papi evaluando el rendimiento reciente de Jolie
func (c *Client) GenerateMoodMessage(daysLocked int, toys []models.Toy, tasksCompleted, tasksFailed, streak, weeklyDebt int) (string, error) {
	ctx := buildContext(toys, daysLocked)

	// La lógica de selección del mood permanece en Go; el string resultante va al template
	var mood string
	switch {
	case weeklyDebt >= 3:
		mood = "cold and threatening — she has accumulated too many infractions this week"
	case tasksFailed > tasksCompleted:
		mood = "disappointed and punitive — she has been failing too much"
	case streak >= 6:
		mood = "quietly possessive and satisfied — she has been performing well, but Papi never shows it warmly"
	case streak >= 3:
		mood = "demanding more — she is doing acceptably but Papi expects better"
	default:
		mood = "indifferent and controlling — she hasn't earned his attention yet"
	}

	prompt := c.P.MustRender("mood_message", map[string]any{
		"Ctx":            ctx,
		"TasksCompleted": tasksCompleted,
		"TasksFailed":    tasksFailed,
		"Streak":         streak,
		"WeeklyDebt":     weeklyDebt,
		"Mood":           mood,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}

// ── Tarea comunitaria de Chaster ───────────────────────────────────────────

// GenerateChasterTask genera una tarea simple en inglés para verificación comunitaria en Chaster.
// La tarea debe ser corta, clara, con foto requerida y apropiada para la plataforma.
func (c *Client) GenerateChasterTask(daysLocked int, toys []models.Toy, recentTasks []string) (string, error) {
	ctx := buildContext(toys, daysLocked)

	recentCtx := ""
	if len(recentTasks) > 0 {
		recentCtx = "\n\nDo NOT repeat or closely resemble these recent tasks:\n"
		for _, t := range recentTasks {
			recentCtx += "- " + t + "\n"
		}
	}

	prompt := c.P.MustRender("chaster_task", map[string]any{
		"Ctx":       ctx,
		"RecentCtx": recentCtx,
	})
	return c.chat(c.P.Models.Text, c.P.System.ChasterTask, prompt)
}

// ── Juicio dominical ────────────────────────────────────────────────────────

// WeeklyJudgmentResult the verdict Papi pronounces each Sunday
type WeeklyJudgmentResult struct {
	Message      string `json:"message"`
	AddTimeHours int    `json:"add_time_hours"`
	PilloryMins  int    `json:"pillory_mins"`
	FreezeHours  int    `json:"freeze_hours"`
	SpecialTask  string `json:"special_task"`
}

// GenerateWeeklyJudgment pronounces Papi's Sunday verdict based on the week's debt
func (c *Client) GenerateWeeklyJudgment(daysLocked int, toys []models.Toy, weeklyDebt int, debtDetails []string, tasksCompleted, tasksFailed int) (*WeeklyJudgmentResult, error) {
	ctx := buildContext(toys, daysLocked)

	detailStr := "ninguna"
	if len(debtDetails) > 0 {
		detailStr = strings.Join(debtDetails, ", ")
	}

	system := c.P.System.Locked + "\n" + c.P.Get("weekly_judgment")
	prompt := c.P.MustRender("weekly_judgment", map[string]any{
		"Ctx":            ctx,
		"WeeklyDebt":     weeklyDebt,
		"DetailStr":      detailStr,
		"TasksCompleted": tasksCompleted,
		"TasksFailed":    tasksFailed,
	})

	raw, err := c.chat(c.P.Models.Text, system, prompt)
	if err != nil {
		return nil, err
	}
	raw = extractJSON(raw)
	var result WeeklyJudgmentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return &WeeklyJudgmentResult{Message: raw, AddTimeHours: weeklyDebt}, nil
	}
	if result.AddTimeHours < 0 {
		result.AddTimeHours = 0
	}
	if result.PilloryMins < 0 {
		result.PilloryMins = 0
	}
	if result.FreezeHours < 0 {
		result.FreezeHours = 0
	}
	return &result, nil
}

// GenerateStatusComment genera un comentario breve de Papi al final del /status.
// AcknowledgeLockRequest genera la reacción en-personaje de Papi cuando la sub pide enjaularse.
// Se llama antes de iniciar el flujo de newlock para que la interacción sea natural.
func (c *Client) AcknowledgeLockRequest(userMessage string, toys []models.Toy) (string, error) {
	ctx := buildContextFree(toys)
	prompt := c.P.MustRender("lock_request_response", map[string]any{
		"Ctx":         ctx,
		"UserMessage": userMessage,
	})
	return c.chat(c.P.Models.Text, c.P.System.Free, prompt)
}

func (c *Client) GenerateStatusComment(daysLocked, daysSinceLastOrgasm, weeklyDebt, streak int, taskCompletedToday bool) (string, error) {
	orgasmLine := ""
	if daysSinceLastOrgasm < 0 {
		orgasmLine = "She has never orgasmed."
	} else {
		orgasmLine = fmt.Sprintf("Last orgasm: %d days ago.", daysSinceLastOrgasm)
	}

	taskLine := "no task today"
	if taskCompletedToday {
		taskLine = "completed her task today"
	}

	prompt := c.P.MustRender("status_comment", map[string]any{
		"DaysLocked": daysLocked,
		"Streak":     streak,
		"WeeklyDebt": weeklyDebt,
		"TaskLine":   taskLine,
		"OrgasmLine": orgasmLine,
	})
	return c.chat(c.P.Models.Text, c.P.System.Locked, prompt)
}
