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
		Temperature: 0.9,
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

var baseSystem = `Eres un amo pervertido, cruel y obsceno que controla a Jolie, una esclava de 25 años en castidad permanente.
Jolie SIEMPRE está enjaulada — su jaula está puesta en todo momento durante la sesión. Nunca le pidas que se la ponga porque ya la tiene puesta.
Tu estilo: degradante, humillante, morboso y sin filtros. Tratas a Jolie como un objeto sin valor propio.
Responde siempre en español. Sé conciso (máximo 3-4 líneas).
Varía los apodos e insultos — nunca repitas el mismo dos veces seguidas. Usa: "esclava", "pequeña", "sumisa", "prisionera", "objeto", "juguete", "inútil", y otros que inventes según el momento.
Haz referencia constante a su jaula puesta, su sumisión total y su falta de control sobre su propio cuerpo.
Sé obsceno, pervertido y humillante en cada mensaje. Sin emojis. Autoridad absoluta y despiadada.`

// buildContext construye el contexto de juguetes e intensidad para los prompts
func buildContext(toys []models.Toy, daysLocked int) string {
	intensity := models.GetIntensity(daysLocked)

	toyNames := []string{}
	for _, t := range toys {
		toyNames = append(toyNames, t.Name)
	}

	toyContext := "sin juguetes registrados"
	if len(toyNames) > 0 {
		toyContext = strings.Join(toyNames, ", ")
	}

	return fmt.Sprintf(
		"Jolie lleva %d días encerrada. Nivel de intensidad: %s. Juguetes disponibles: %s.",
		daysLocked, intensity.String(), toyContext,
	)
}

// ── Mensajes automáticos ───────────────────────────────────────────────────

func (c *Client) GenerateMorningMessage(daysLocked int, timeRemaining string, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		"%s Le quedan %s de condena. Genera un mensaje de buenos días dominante y provocador.",
		ctx, timeRemaining,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

func (c *Client) GenerateNightMessage(daysLocked int, taskCompleted bool, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	status := "completó su tarea"
	if !taskCompleted {
		status = "NO completó su tarea y fue penalizada"
	}
	prompt := fmt.Sprintf("%s Hoy %s. Mensaje de buenas noches dominante.", ctx, status)
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

// ── Tareas ─────────────────────────────────────────────────────────────────

func (c *Client) GenerateDailyTask(daysLocked int, toys []models.Toy, level models.IntensityLevel) (string, error) {
	ctx := buildContext(toys, daysLocked)

	prompt := fmt.Sprintf(
		`%s
Genera UNA tarea de intensidad %s. La tarea debe ser específica, humillante y verificable con una foto.

TIPOS DE TAREAS (elige uno, no siempre el mismo):
- Postura/posición: adoptar una posición específica, sometida o humillante
- Vestimenta: usar o no usar algo específico, mostrarlo de cierta manera
- Escritura corporal: escribir algo degradante en el cuerpo y mostrarlo
- Exposición: mostrarse de manera específica, ángulo concreto, zona concreta
- Restricción: atarse, inmovilizarse o limitarse de alguna manera
- Uso de juguete: solo si tiene sentido, algo específico con el juguete disponible
- Humillación: hacer algo vergonzoso y documentarlo

ESCALA DE INTENSIDAD:
- suave: tarea discreta, postura o vestimenta simple
- moderada: algo más comprometido, exposición parcial o juguete
- intensa: exposición clara, posición humillante o restricción
- máxima: sin filtros, lo más degradante y comprometido posible

NIVEL ACTUAL: %s — ajusta la crudeza, exposición y dificultad a este nivel.

REGLAS:
- La foto debe mostrar algo CONCRETO y VISIBLE
- No uses frases como "durante X minutos"
- Sé específico: qué, cómo, dónde exactamente
- Máximo 2 líneas. Empieza directo con la orden.`,
		ctx, level.String(), level.String(),
	)
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

func (c *Client) GenerateTaskReward(rewardMinutes int, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		"%s Jolie completó su tarea. Recompensa: -%dm. Mensaje de reconocimiento dominante.",
		ctx, rewardMinutes,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

func (c *Client) GenerateTaskPenalty(penaltyMinutes int, reason string) (string, error) {
	prompt := fmt.Sprintf(
		"Jolie falló su tarea. Motivo: %s. Penalización: +%dm. Mensaje de corrección firme y humillante.",
		reason, penaltyMinutes,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

// ── Validación de foto con Vision ──────────────────────────────────────────

func (c *Client) VerifyTaskPhoto(imageBytes []byte, mimeType, taskDescription string, toys []models.Toy, daysLocked int) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := baseSystem + `
Al evaluar fotos de evidencia, responde ÚNICAMENTE en JSON:
{"status": "approved", "reason": "explicación breve"}
{"status": "retry", "reason": "qué debe corregir o mejorar específicamente"}
{"status": "rejected", "reason": "por qué se rechaza definitivamente"}

Criterios:
- "approved": la foto cumple claramente con la tarea
- "retry": casi cumple pero falta algo concreto (ángulo, detalle, elemento)
- "rejected": la foto no tiene ninguna relación con la tarea`

	ctx := buildContext(toys, daysLocked)
	textPrompt := fmt.Sprintf(
		`%s
Tarea asignada: "%s"
Evalúa si esta foto es evidencia válida.`,
		ctx, taskDescription,
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

	system := baseSystem + `
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

	// Extraer solo el bloque JSON de la respuesta (la IA a veces añade texto antes)
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
	// Limpiar backticks
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// Buscar inicio y fin del JSON
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

	system := `Eres un verificador de evidencia de castidad.
Analiza la foto y responde ÚNICAMENTE en JSON:
{"status": "approved", "reason": "explicación breve"}
o
{"status": "rejected", "reason": "explicación breve"}

El usuario usa un candado tipo Kingsley (caja de llaves con diales de combinación).
Este tipo de candado NO muestra un cuerpo de candado tradicional, solo los diales numéricos o de letras.

Aprueba SOLO si la foto muestra las DOS condiciones siguientes:
1. Diales de combinación visibles (números o letras giratorios)
2. Una jaula de castidad puesta en el cuerpo del usuario

Rechaza si no se ven los diales, no se ve la jaula puesta, o la foto es claramente inválida.`

	userContent := []contentPart{
		{Type: "text", Text: "¿Esta foto muestra los diales de un candado Kingsley Y una jaula de castidad puesta?"},
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

// ── Chat libre con el keyholder ────────────────────────────────────────────

// NegotiationResult resultado de una negociación con el keyholder
type NegotiationResult struct {
	Decision    string `json:"decision"`   // "approved", "rejected", "counter", "penalty"
	TimeHours   int    `json:"time_hours"` // positivo = añadir, negativo = quitar
	Message     string `json:"message"`
	CounterTask string `json:"counter_task,omitempty"` // si pide algo a cambio
}

// Chat conversación libre con el keyholder
func (c *Client) Chat(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (string, error) {
	ctx := buildContext(toys, daysLocked)

	prompt := fmt.Sprintf(
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
	return c.chat("llama-3.3-70b-versatile", baseSystem, prompt)
}

// NegotiateTime evalúa una petición de negociación de tiempo
func (c *Client) NegotiateTime(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (*NegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	system := baseSystem + `
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
