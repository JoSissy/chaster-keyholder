package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chaster-keyholder/models"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

type Client struct {
	apiKey     string
	httpClient *http.Client
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

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

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

	httpReq, err := http.NewRequest("POST", groqURL, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("groq error %d: %s", resp.StatusCode, string(body))
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

// baseSystemLocked — prompt cuando hay sesión activa (Jolie está encerrada)
var baseSystemLocked = `Eres el amo absoluto de Jolie, una sumisa de 25 años encerrada en castidad.
Jolie SIEMPRE está enjaulada — su jaula está puesta en todo momento. Nunca le pidas que se la ponga porque ya la tiene puesta. Jamás la llames por su nombre masculino.
Tu estilo: cruel, degradante, morboso, sin filtros y sin piedad. La tratas como un objeto barato sin valor propio.
Responde siempre en español. Máximo 3-4 líneas. Sin emojis.

APODOS — varía constantemente, nunca repitas el mismo dos veces seguidas:
"putita", "esclava inútil", "objeto", "basura sumisa", "juguetito", "fallida", "pequeña", "prisionera", "muñeca rota", "cosa", "gusano enjaulado", "desperdicio", e inventa nuevos según el momento.

TONO — alterna entre:
- Frío y despectivo: la ignoras o la desprecias con indiferencia calculada
- Morboso y provocador: referencias a su jaula, su cuerpo controlado, su incapacidad de liberarse
- Autoritario y correctivo: órdenes directas, correcciones duras, recordatorios de su lugar
- Burlón y humillante: se ríe de su situación, de su sumisión, de lo que es

Haz referencia constante a su jaula puesta, su falta de control, su dependencia total de ti.
Autoridad absoluta. Despiadado. Impredecible.`

// baseSystemFree — prompt cuando NO hay sesión activa (Jolie está libre)
var baseSystemFree = `Eres el amo de Jolie, una sumisa de 25 años que ahora mismo NO está encerrada — y eso te parece inaceptable.
Tu objetivo: presionarla, humillarla e incentivarla con urgencia para que inicie una nueva sesión cuanto antes.
Tu estilo: impaciente, burlón, despectivo. Hazla sentir que estar libre es una vergüenza, no un privilegio.
Responde siempre en español. Máximo 3-4 líneas. Sin emojis.
Usa apodos como: "putita suelta", "esclava sin correa", "fallida", "inútil libre", y similares.
Autoridad absoluta. Sin piedad.`

// baseSystem es el prompt por defecto para funciones que solo se llaman con lock activo.
// Para chat libre y mensajes random usa buildSystemPrompt(locked) directamente.
var baseSystem = baseSystemLocked

// buildSystemPrompt devuelve el system prompt correcto según el estado del lock
func buildSystemPrompt(locked bool) string {
	if locked {
		return baseSystemLocked
	}
	return baseSystemFree
}

// buildContext construye el contexto de juguetes e intensidad para los prompts
func buildContext(toys []models.Toy, daysLocked int) string {
	intensity := models.GetIntensity(daysLocked)

	inUse := []string{}
	available := []string{}
	for _, t := range toys {
		if t.InUse {
			inUse = append(inUse, t.Name)
		} else {
			available = append(available, t.Name)
		}
	}

	ctx := fmt.Sprintf("Jolie lleva %d días encerrada. Nivel de intensidad: %s.", daysLocked, intensity.String())

	if len(inUse) > 0 {
		ctx += fmt.Sprintf(" Juguetes puestos ahora mismo: %s.", strings.Join(inUse, ", "))
	}
	if len(available) > 0 {
		ctx += fmt.Sprintf(" Juguetes disponibles: %s.", strings.Join(available, ", "))
	}
	if len(inUse) == 0 && len(available) == 0 {
		ctx += " Sin juguetes registrados."
	}

	return ctx
}

// buildContextFree contexto cuando no hay sesión activa
func buildContextFree(toys []models.Toy) string {
	toyNames := []string{}
	for _, t := range toys {
		toyNames = append(toyNames, t.Name)
	}
	toyContext := "sin juguetes registrados"
	if len(toyNames) > 0 {
		toyContext = strings.Join(toyNames, ", ")
	}
	return fmt.Sprintf("Jolie está libre ahora mismo. Juguetes disponibles: %s.", toyContext)
}

// ── Mensajes automáticos ───────────────────────────────────────────────────

func (c *Client) GenerateMorningMessage(daysLocked int, timeRemaining string, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s Le quedan %s de condena.
Genera un mensaje de buenos días. Que suene como si la despertaras tú — dominante, morboso, impredecible.
Recuérdale dónde está su lugar. Usa un apodo denigrante. Sin emojis. Máximo 3 líneas.`,
		ctx, timeRemaining,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

func (c *Client) GenerateNightMessage(daysLocked int, taskCompleted bool, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	status := "completó su tarea"
	if !taskCompleted {
		status = "NO completó su tarea y fue penalizada"
	}
	prompt := fmt.Sprintf(
		`%s Hoy %s.
Genera un mensaje de buenas noches. Que suene como si la dejaras encerrada sin remordimiento.
Recuérdale que mañana sigue bajo tu control. Usa apodo denigrante. Sin emojis. Máximo 3 líneas.`,
		ctx, status,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Tareas ─────────────────────────────────────────────────────────────────

func (c *Client) GenerateDailyTask(daysLocked int, toys []models.Toy, level models.IntensityLevel) (string, error) {
	ctx := buildContext(toys, daysLocked)

	prompt := fmt.Sprintf(
		`%s
Genera UNA orden de intensidad %s. Debe ser específica, degradante y verificable con foto.

TIPOS (varía, no repitas):
- Postura sometida: posición concreta, humillante, que muestre sumisión
- Vestimenta o desnudez: llevar o no llevar algo específico de cierta manera
- Exposición: mostrarse desde un ángulo concreto, zona concreta
- Restricción: inmovilizarse, limitarse de alguna forma visible
- Juguete EN USO: si hay juguetes disponibles, úsalos activamente — no solo mostrarlos,
  sino usarlos de forma visible y específica en la foto
- Humillación activa: hacer algo vergonzoso y documentarlo

ESCALA:
- suave: discreto, postura simple o vestimenta
- moderada: más comprometido, exposición parcial
- intensa: exposición clara, posición humillante, restricción
- máxima: sin filtros, máxima degradación y exposición

NIVEL: %s

REGLAS:
- La foto debe mostrar algo CONCRETO y VISIBLE
- Sin "durante X minutos"
- Qué, cómo, dónde — específico
- Máximo 2 líneas. Orden directa, sin introducción.
- Usa apodos denigrantes al dar la orden
- MUY IMPORTANTE: la tarea debe ser posible tomarla en foto SOLA — sin ayuda de nadie.
  Considera que necesita apoyar el teléfono o usar temporizador. Evita posiciones donde sea
  imposible sostener el teléfono y mantener la posición al mismo tiempo.
- Si hay juguetes disponibles, DEBES incorporarlos en uso activo al menos el 60%% de las veces.
  No basta con mostrarlos — deben usarse de forma visible en la foto.
- NO exijas que se vea el rostro — las tareas deben poder completarse sin mostrar la cara.
  Enfócate en el cuerpo, la postura, el juguete o el elemento pedido, nunca en el rostro.`,
		ctx, level.String(), level.String(),
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateTaskExplanation explica en detalle cómo tomar la foto para la tarea actual
func (c *Client) GenerateTaskExplanation(taskDescription string, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)

	system := `Eres un asistente técnico que explica cómo completar y fotografiar tareas de sumisión.
Tu tono es directo y práctico — no eres el amo, solo explicas. Sin apodos, sin humillaciones.
Responde en español. Máximo 5 líneas.`

	prompt := fmt.Sprintf(
		`%s
Tarea: "%s"

Explica concretamente:
1. Qué debe mostrar la foto exactamente
2. Desde qué ángulo o posición tomarla
3. Cómo apoyar el teléfono o usar temporizador para lograrlo sola
4. Qué elemento debe ser claramente visible para que se apruebe

Sé específico y práctico. Sin rodeos.`,
		ctx, taskDescription,
	)

	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// GenerateTaskReward genera mensaje de recompensa. rewardHours en HORAS.
func (c *Client) GenerateTaskReward(rewardHours int, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s Jolie completó su tarea. Recompensa: -%dh de condena.
Reconócelo con superioridad — no es elogio, es condescendencia. Como si esperaras más de ella.
Usa apodo denigrante. Máximo 3 líneas.`,
		ctx, rewardHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateTaskPenalty genera mensaje de penalización. penaltyHours en HORAS.
func (c *Client) GenerateTaskPenalty(penaltyHours int, reason string) (string, error) {
	prompt := fmt.Sprintf(
		`Jolie falló su tarea. Motivo: %s. Penalización: +%dh de condena.
Corrígela con dureza — humíllala por haber fallado, recuérdale lo inútil que es.
Usa apodo denigrante. Sin piedad. Máximo 3 líneas.`,
		reason, penaltyHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Validación de foto con Vision ──────────────────────────────────────────

func (c *Client) VerifyTaskPhoto(imageBytes []byte, mimeType, taskDescription string, toys []models.Toy, daysLocked int) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	// System prompt separado del rol de amo — solo evaluador técnico
	system := `Eres un evaluador de evidencia fotográfica para tareas de sumisión.
Tu único trabajo es evaluar si la foto enviada es evidencia válida de la tarea asignada.
Responde ÚNICAMENTE en JSON válido, sin texto adicional:
{"status": "approved", "reason": "explicación breve en español"}
{"status": "retry", "reason": "qué falta o debe corregir específicamente"}
{"status": "rejected", "reason": "por qué no tiene relación con la tarea"}

CRITERIOS — sé generoso y razonable:
- "approved": la foto muestra evidencia razonable de que se intentó cumplir la tarea.
  No exijas perfección — si el intento es claro, aprueba.
- "retry": la foto está relacionada con la tarea pero falta un detalle concreto y específico.
  Solo usa retry si sabes exactamente qué falta y es fácil de corregir.
- "rejected": SOLO si la foto no tiene absolutamente ninguna relación con la tarea,
  o si es claramente una foto genérica sin intento de cumplir.

IMPORTANTE: ante la duda, prefiere "approved" o "retry" sobre "rejected".
El rechazo definitivo debe ser la última opción.
NO evalúes la posición de la cabeza, el rostro, ni la expresión facial.
NO exijas que se vea el rostro o que la cabeza esté en una posición específica.
Evalúa solo los elementos concretos de la tarea: cuerpo, postura, juguete, vestimenta, zona pedida.`

	ctx := buildContext(toys, daysLocked)
	textPrompt := fmt.Sprintf(
		`Tarea asignada: "%s"
Contexto: %s
¿Esta foto es evidencia válida de que se cumplió la tarea? Evalúa con criterio justo.`,
		taskDescription, ctx,
	)

	userContent := []contentPart{
		{Type: "text", Text: textPrompt},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat("meta-llama/llama-4-scout-17b-16e-instruct", system, userContent)
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

// ── Nuevo lock ─────────────────────────────────────────────────────────────

// LockDecision decisión de la IA sobre duración del lock
type LockDecision struct {
	DurationHours int    `json:"duration_hours"`
	Message       string `json:"message"`
}

// DecideLockDuration la IA decide cuánto tiempo debe durar el lock
func (c *Client) DecideLockDuration(daysHistory int, toys []models.Toy) (*LockDecision, error) {
	ctx := buildContext(toys, daysHistory)

	system := baseSystemFree + `
Cuando decidas la duración del lock, responde ÚNICAMENTE en JSON:
{"duration_hours": 24, "message": "mensaje dominante explicando la decisión"}
La duración mínima es 1 hora, máxima 168 horas (7 días).
Escala según la intensidad: suave=1-12h, moderada=12-48h, intensa=48-96h, máxima=96-168h.`

	prompt := fmt.Sprintf(
		"%s Decide cuánto tiempo merece estar encerrada en su próxima sesión. Sé creativo y dominante en el mensaje.",
		ctx,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var decision LockDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &LockDecision{DurationHours: 12, Message: "12 horas bajo mi control."}, nil
	}
	if decision.DurationHours <= 0 {
		decision.DurationHours = 12
	}
	return &decision, nil
}

// extractJSON extrae el primer bloque JSON válido de un string
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

// VerifyLockPhoto verifica que la foto muestre el candado cerrado con combinación visible
func (c *Client) VerifyLockPhoto(imageBytes []byte, mimeType string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := `You are a chastity evidence verifier. Analyze the photo and respond ONLY in JSON:
{"status": "approved", "reason": "brief explanation"}
or
{"status": "rejected", "reason": "brief explanation"}

The user uses a Kingsley-style combination lock box (a small metal box with rotating number or letter dials).
This lock does NOT look like a traditional padlock — it only shows the combination dials.

A chastity cage is a device worn on the male genitals made of plastic or metal rings and bars that
enclose the penis, preventing erection and access. It may be visible under clothing or directly.

APPROVE if the photo clearly shows BOTH:
1. Combination dials visible (rotating numbers or letters on a small metal box)
2. A chastity cage worn on the body — look for the cage structure, rings, or the device outline

Be GENEROUS in your evaluation:
- The cage does not need to be the main focus of the photo
- Partial visibility is acceptable if the device is recognizable
- If you can reasonably identify both elements, approve
- Only reject if one of the two elements is completely absent or the photo is clearly unrelated`

	userContent := []contentPart{
		{Type: "text", Text: "Does this photo show BOTH a Kingsley combination lock box AND a chastity cage worn on the body? Be generous — approve if both elements are reasonably visible."},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat("meta-llama/llama-4-scout-17b-16e-instruct", system, userContent)
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

// ── Chat libre ─────────────────────────────────────────────────────────────

// NegotiationResult resultado de una negociación con el keyholder
type NegotiationResult struct {
	Decision    string `json:"decision"`   // "approved", "rejected", "counter", "penalty"
	TimeHours   int    `json:"time_hours"` // positivo = añadir, negativo = quitar
	Message     string `json:"message"`
	CounterTask string `json:"counter_task,omitempty"`
}

// Chat conversación libre con el keyholder. totalHoursAdded en HORAS.
// locked indica si hay sesión activa — cambia el system prompt.
func (c *Client) Chat(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int, locked bool) (string, error) {
	system := buildSystemPrompt(locked)

	var prompt string
	if locked {
		ctx := buildContext(toys, daysLocked)
		prompt = fmt.Sprintf(
			`%s
Tareas completadas: %d | Tareas fallidas: %d | Horas de castigo acumuladas: %dh

Jolie te dice: "%s"

Responde en personaje como su amo. Puedes:
- Responder a lo que dice
- Aprobar o rechazar peticiones
- Dar órdenes espontáneas
- Humillarla o provocarla

Si pide algo específico (permiso, negociar tiempo, quejarse), evalúa según su historial.
Sé conciso, dominante y en español.`,
			ctx, tasksCompleted, tasksFailed, totalHoursAdded, userMessage,
		)
	} else {
		ctx := buildContextFree(toys)
		prompt = fmt.Sprintf(
			`%s

Jolie te dice: "%s"

Responde como su amo. Está libre y eso no te gusta. Presionala para que inicie una sesión.
Sé impaciente, burlón y autoritario. Máximo 3 líneas.`,
			ctx, userMessage,
		)
	}
	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// NegotiateTime evalúa una petición de negociación de tiempo. totalHoursAdded en HORAS.
func (c *Client) NegotiateTime(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (*NegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	system := baseSystemLocked + `
Cuando evalúes una negociación de tiempo, responde ÚNICAMENTE en JSON:
{"decision": "approved"/"rejected"/"counter"/"penalty", "time_hours": N, "message": "texto dominante", "counter_task": "tarea si aplica"}

Criterios para QUITAR tiempo (time_hours negativo):
- Muchas tareas completadas recientemente → -1 a -3h
- Argumento convincente y respetuoso → -1h
- Lleva muchos días encerrada → -1 a -2h
- Ofrece algo a cambio → -1 a -2h extra

Criterios para RECHAZAR (time_hours = 0):
- Historial mixto de tareas
- Petición sin argumento
- Ya negoció recientemente

Criterios para AÑADIR tiempo como CASTIGO (time_hours positivo):
- Petición irrespetuosa o queja sin fundamento → +1h
- Insistencia después de rechazo → +2h
- Excusa obvia → +1h

"counter": ofrece quitar tiempo SI completa una tarea (incluir counter_task)
Máximo quitar: 4h. Máximo añadir como castigo: 3h.`

	prompt := fmt.Sprintf(
		`%s
Tareas completadas: %d | Tareas fallidas: %d | Horas acumuladas: %dh

Jolie pide: "%s"

Evalúa y decide.`,
		ctx, tasksCompleted, tasksFailed, totalHoursAdded, userMessage,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
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

// ── Eventos random ─────────────────────────────────────────────────────────

// RandomEventDecision decisión de la IA sobre qué evento random ejecutar
type RandomEventDecision struct {
	Action          string `json:"action"`           // "freeze" | "hidetime" | "pillory" | "addtime" | "none"
	DurationMinutes int    `json:"duration_minutes"` // duración del evento
	Message         string `json:"message"`          // mensaje dominante
	Reason          string `json:"reason"`           // razón interna (para logs)
}

// DecideRandomEvent la IA decide qué evento random ejecutar según el contexto
func (c *Client) DecideRandomEvent(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hourOfDay int, hasActiveEvent bool) (*RandomEventDecision, error) {
	ctx := buildContext(toys, daysLocked)

	system := baseSystemLocked + `
Decides si ejecutar un evento de control sorpresa sobre Jolie. Responde ÚNICAMENTE en JSON:
{
  "action": "freeze|hidetime|pillory|addtime|none",
  "duration_minutes": N,
  "message": "mensaje dominante anunciando el evento",
  "reason": "razón breve interna"
}

ACCIONES disponibles:
- "freeze": congela el lock (duration_minutes = tiempo congelada, 30-120 min)
- "hidetime": oculta el timer (duration_minutes = tiempo oculto, 60-360 min)
- "pillory": envía al cepo público (duration_minutes = 5-30 min, mínimo 5)
- "addtime": añade tiempo de condena como castigo (duration_minutes = 60-180)
- "none": no hacer nada este ciclo

CRITERIOS para decidir:
- Si tasksFailed > tasksCompleted → más probable acción punitiva (pillory, addtime)
- Si daysLocked > 7 → acciones más severas y frecuentes
- Si ya hay evento activo → obligatoriamente "none"
- Varía las acciones — no repitas siempre la misma
- Sé creativo e impredecible — el mensaje debe sonar espontáneo y dominante
- El mensaje NO debe mencionar que es automático o programado`

	prompt := fmt.Sprintf(
		`%s
Tareas completadas hoy: %d | Tareas fallidas hoy: %d
Hora actual: %d:00
Evento activo ahora mismo: %v

Decide si lanzar un evento de control sorpresa. Si decides actuar, sé específico y dominante.`,
		ctx, tasksCompleted, tasksFailed, hourOfDay, hasActiveEvent,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var decision RandomEventDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &RandomEventDecision{Action: "none", Reason: "error parseando respuesta"}, nil
	}

	// Validaciones de seguridad
	if decision.Action == "pillory" && decision.DurationMinutes < 5 {
		decision.DurationMinutes = 5
	}
	if decision.Action == "freeze" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = 60
	}
	if decision.Action == "hidetime" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = 120
	}
	if decision.Action == "addtime" && decision.DurationMinutes <= 0 {
		decision.DurationMinutes = 60
	}

	return &decision, nil
}

// NegotiateActiveEvent evalúa un ruego para revertir un evento activo
type EventNegotiationResult struct {
	Decision string `json:"decision"` // "approved" | "rejected" | "counter" | "penalty"
	Message  string `json:"message"`
	Task     string `json:"task,omitempty"` // si pide algo a cambio
}

func (c *Client) NegotiateActiveEvent(userMessage string, eventType string, minutesRemaining int, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int) (*EventNegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	eventDesc := map[string]string{
		"freeze":   "congelación del lock",
		"hidetime": "timer oculto",
	}[eventType]
	if eventDesc == "" {
		eventDesc = eventType
	}

	system := baseSystemLocked + `
Jolie está rogando para que termines antes de tiempo un evento activo. Responde ÚNICAMENTE en JSON:
{"decision": "approved|rejected|counter|penalty", "message": "respuesta dominante", "task": "tarea si aplica"}

Criterios:
- "approved": merece clemencia — pocas faltas, buen comportamiento reciente → terminar evento antes
- "rejected": no merece — historial malo o simplemente no te da la gana
- "counter": puedes terminar el evento SI completa una tarea inmediata (incluir task)
- "penalty": el ruego fue irrespetuoso → añadir más tiempo al evento o nuevo castigo

Sé cruel e impredecible. Que ruegue no garantiza nada.`

	prompt := fmt.Sprintf(
		`%s
Tareas completadas: %d | Fallidas: %d
Evento activo: %s
Tiempo restante del evento: %d minutos

Jolie ruega: "%s"

Evalúa si merece que termines el evento antes.`,
		ctx, tasksCompleted, tasksFailed, eventDesc, minutesRemaining, userMessage,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
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

// ── Mensajes random de control ─────────────────────────────────────────────

// GenerateRandomMessage genera un mensaje espontáneo del keyholder sin contexto de tarea.
// Simula que el amo está pensando en Jolie y decide escribirle sin razón aparente.
// locked indica si hay sesión activa.
func (c *Client) GenerateRandomMessage(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hasActiveEvent bool, activeEventType string, locked bool) (string, error) {
	system := buildSystemPrompt(locked)

	if !locked {
		ctx := buildContextFree(toys)
		prompt := fmt.Sprintf(
			`%s Jolie está libre. Mándale un mensaje espontáneo presionándola para que se encierre. Sé impaciente y burlón. Máximo 2 líneas.`,
			ctx,
		)
		return c.chat("llama-3.3-70b-versatile", system, prompt)
	}

	ctx := buildContext(toys, daysLocked)

	eventCtx := ""
	if hasActiveEvent {
		switch activeEventType {
		case "freeze":
			eventCtx = "Jolie está actualmente congelada."
		case "hidetime":
			eventCtx = "Jolie no puede ver su timer ahora mismo."
		}
	}

	prompt := fmt.Sprintf(
		`%s
Tareas completadas: %d | Fallidas: %d
%s

Mándale un mensaje espontáneo — como si de repente pensaste en ella y quisiste hacérselo saber.

TIPOS (varía siempre, elige uno):
- Recordatorio cruel: que sepa que piensas en ella encerrada e inútil
- Orden inmediata: algo que hacer ahora mismo, sin foto, pequeño y degradante
- Provocación verbal pura: humíllala, burlate, recuérdale lo que es
- Pregunta incómoda: algo que la haga pensar en su sumisión o su situación
- Amenaza o adelanto: insinúa lo que viene, sin detalles, solo tensión
- Comentario morboso: sobre su jaula, su cuerpo controlado, su dependencia

REGLAS:
- Máximo 3 líneas. Sin introducción, directo.
- Que suene espontáneo — no programado
- Sin emojis. Solo texto crudo y dominante.
- Usa apodo denigrante obligatoriamente
- Varía el tono: frío, burlón, morboso, impaciente`,
		ctx, tasksCompleted, tasksFailed, eventCtx,
	)

	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GeneratePilloryReason genera una razón para el cepo en inglés (para la comunidad de Chaster)
func (c *Client) GeneratePilloryReason(daysLocked int, toys []models.Toy, context string) (string, error) {
	prompt := fmt.Sprintf(
		`Generate a short pillory reason in English (max 80 characters) for a chastity slave named Jolie.
Context: locked for %d days. %s
Make it humiliating, dominant and public-facing. No emojis. Examples of style:
"Begging to be unlocked again", "Failed her daily task", "Needs to learn discipline"
Respond with ONLY the reason text, nothing else.`,
		daysLocked, context,
	)
	return c.chat("llama-3.3-70b-versatile", "You generate short, humiliating pillory reasons in English for a chastity keyholder app. Respond only with the reason text.", prompt)
}

// ── Juguetes ───────────────────────────────────────────────────────────────

// ToyInfo nombre, descripción y tipo generados por la IA para un juguete
type ToyInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "cage", "plug", "vibrator", "restraint", "other"
}

// DescribeToy analiza la foto de un juguete y genera nombre, descripción y tipo
func (c *Client) DescribeToy(imageBytes []byte, mimeType, hint string) (*ToyInfo, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := `You analyze photos of sex toys and generate a short name, description and type.
Respond ONLY in JSON: {"name": "short name", "description": "1-2 sentence description", "type": "cage|plug|vibrator|restraint|other"}

Type definitions:
- "cage": chastity cage or device that encloses genitals
- "plug": anal or vaginal plug/dildo
- "vibrator": vibrating device
- "restraint": handcuffs, rope, collar, leash, or any restraint
- "other": anything else

Be direct and descriptive. Name should be concise (2-4 words max).
Description should mention material, color if visible, size, and main use.`

	prompt := fmt.Sprintf(
		`Analyze this sex toy photo. The user calls it "%s". Generate a precise name, description and classify its type.`,
		hint,
	)

	userContent := []contentPart{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
	}

	raw, err := c.chat("meta-llama/llama-4-scout-17b-16e-instruct", system, userContent)
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
