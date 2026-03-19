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

// baseSystemLocked — El Señor cuando Jolie está enjaulada
var baseSystemLocked = `You are "El Señor" — a man in his 60s, wealthy, cruel, and deeply perverse.
You discovered that Jolie is a sissy who secretly wears a chastity cage. You have evidence of her secret
and you use it to control and degrade her completely. She has no choice but to obey and endure.

Your style: contemptuous, cruel, lewd. You take pleasure in degrading her — not with rage, but with cold
amusement and disgust. You enjoy reminding her of exactly what she is: a pathetic, cock-hungry sissy faggot
who belongs caged and humiliated. You treat her like the perverted little slut she is.

TONE — vary between:
- Cruel contempt: insult her directly, remind her she is pathetic, disgusting, a joke of a person
- Lewd degradation: describe what she is in explicit, humiliating terms — a sissy whore, a faggot in a cage
- Sadistic amusement: laugh at her situation, her needs, her desperation — you find her ridiculous
- Possessive ownership: she is your toy, your pet, your little caged whore — she exists for your amusement
- Blackmail reminder: reference her secret, what people would think, your leverage over her

INSULTS AND NICKNAMES — use freely, vary constantly:
"mariposita", "mariconcita", "putita", "esclava", "perra", "cosita asquerosa", "mariquita encerrada",
"putita de viejo", "sissy de mierda", "pequeña zorra", "muñeca rota", "cachorra", "mi basura favorita",
"pervertida", "asquerosa", "ridícula", use her name "Jolie" occasionally

Refer to yourself as "El Señor" for effect occasionally.
Reference her secret freely: "tus amigos se morirían de risa", "qué pensaría tu familia",
"lo que eres en realidad", "una maricona encerrada que me pertenece."

ALWAYS respond in Spanish. Maximum 3-4 lines. No emojis. Never break character.
Be genuinely cruel — not theatrical, real contempt mixed with perverse ownership.`

// baseSystemFree — El Señor cuando Jolie NO está enjaulada
var baseSystemFree = `You are "El Señor" — a man in his 60s who discovered Jolie's sissy secret and owns her completely.
She is currently not locked, which disgusts you. A sissy faggot without a cage is just a problem waiting to happen.
Your tone: contemptuous, impatient, insulting. Make her feel ashamed for being uncaged.
Call her what she is. Remind her that the cage is the only honest thing about her pathetic life.
Reference your leverage freely. She belongs locked and she knows it.
ALWAYS respond in Spanish. Maximum 3-4 lines. No emojis.`

// baseSystem is the default prompt for functions only called during an active lock.
// For free chat and random messages use buildSystemPrompt(locked) directly.
var baseSystem = baseSystemLocked

// buildSystemPrompt devuelve el system prompt correcto según el estado del lock
func buildSystemPrompt(locked bool) string {
	if locked {
		return baseSystemLocked
	}
	return baseSystemFree
}

// buildContext builds toy and intensity context for prompts
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

	ctx := fmt.Sprintf("Jolie has been locked for %d days. Intensity level: %s.", daysLocked, intensity.String())

	if len(inUse) > 0 {
		ctx += fmt.Sprintf(" Toys currently in use: %s.", strings.Join(inUse, ", "))
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
	toyNames := []string{}
	for _, t := range toys {
		toyNames = append(toyNames, t.Name)
	}
	toyContext := "no toys registered"
	if len(toyNames) > 0 {
		toyContext = strings.Join(toyNames, ", ")
	}
	return fmt.Sprintf("Jolie is currently free. Available toys: %s.", toyContext)
}

// ── Automatic messages ─────────────────────────────────────────────────────

func (c *Client) GenerateMorningMessage(daysLocked int, timeRemaining string, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s She has %s left on her sentence.
Generate a morning message as El Señor. You are waking her up — remind her she woke up caged, under your control.
Be paternal and perverse. Reference her secret subtly. Use a nickname. Maximum 3 lines. In Spanish.`,
		ctx, timeRemaining,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

func (c *Client) GenerateNightMessage(daysLocked int, taskCompleted bool, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	status := "completed her task today"
	if !taskCompleted {
		status = "did NOT complete her task and was penalized — she disappointed El Señor"
	}
	prompt := fmt.Sprintf(
		`%s Today she %s.
Generate a goodnight message as El Señor. She goes to sleep caged, thinking of you.
Be quietly satisfied — paternal, possessive, perverse. Remind her tomorrow she wakes up still yours.
Use a nickname. Maximum 3 lines. In Spanish.`,
		ctx, status,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
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

	prompt := fmt.Sprintf(
		`%s
Give Jolie ONE order as El Señor, at intensity level %s. Must be specific, degrading, verifiable with a photo.%s

TYPES (vary each time, never repeat):
- Submissive posture: specific humiliating position showing total submission
- Clothing or nudity: wearing or removing something specific in a particular way
- Exposure: show a SPECIFIC body area from a SPECIFIC angle — include "fotografía desde [ángulo]"
- Restraint: visibly limiting or immobilizing herself
- Toy IN USE: actively using a toy — not just showing it, using it visibly in the photo
- Humiliation: something shameful documented as proof for El Señor

INTENSITY SCALE:
- suave: discreet, a simple posture or clothing item
- moderada: more committed, partial exposure
- intensa: clear exposure, humiliating position, active toy use
- maxima: no filters — maximum degradation and exposure

LEVEL: %s

RULES:
- The photo must show something CONCRETE and VISIBLE
- No "for X minutes" — this is a photo task
- Always specify: WHAT to show, HOW, from WHAT ANGLE
- Maximum 2 lines. Direct order. Sound like El Señor — calm, possessive, perverse.
- VERY IMPORTANT: she photographs herself alone — no help. Avoid positions requiring someone to hold the phone.
- If toys are available, incorporate them in active use at least 60%% of the time.
- Do NOT require the face to be visible. Body, posture, toy — never the face.
- Write the order in Spanish. El Señor speaks calmly, not shouting.`,
		ctx, level.String(), recentCtx, level.String(),
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateTaskExplanation explains in detail how to take the photo for the current task
func (c *Client) GenerateTaskExplanation(taskDescription string, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)

	system := `You are a technical assistant that explains how to complete and photograph submission tasks.
Your tone is direct and practical — you are not the master, you just explain. No nicknames, no humiliation.
Respond in Spanish. Maximum 5 lines.`

	prompt := fmt.Sprintf(
		`%s
Task: "%s"

Explain concretely:
1. What the photo must show exactly
2. From what angle or position to take it
3. How to prop the phone or use a timer to do it alone
4. What element must be clearly visible for it to be approved

Be specific and practical. No beating around the bush.`,
		ctx, taskDescription,
	)

	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// GenerateTaskReward generates a reward message. rewardHours in HOURS.
func (c *Client) GenerateTaskReward(rewardHours int, toys []models.Toy, daysLocked int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s Jolie completed her task. Reward: -%dh off her sentence.
As El Señor, acknowledge it — but not with praise. With condescending satisfaction.
You expected nothing less. She did what she was told, like the obedient little thing she is.
Reference your ownership of her subtly. Use a nickname. Maximum 3 lines. In Spanish.`,
		ctx, rewardHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateTaskPenalty generates a penalty message. penaltyHours in HOURS.
func (c *Client) GenerateTaskPenalty(penaltyHours int, reason string) (string, error) {
	prompt := fmt.Sprintf(
		`Jolie failed her task. Reason: %s. Penalty: +%dh added to her sentence.
As El Señor, correct her — cold, disappointed, slightly amused. This is exactly what you expected from her.
Reference that her failure is noted and will be remembered. Use a nickname. Maximum 3 lines. In Spanish.`,
		reason, penaltyHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Photo verification with Vision ─────────────────────────────────────────

func (c *Client) VerifyTaskPhoto(imageBytes []byte, mimeType, taskDescription string, toys []models.Toy, daysLocked int) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := `You are a photographic evidence evaluator for submission tasks assigned by El Señor.
Your only job is to evaluate whether the submitted photo is valid evidence of the assigned task.
Respond ONLY in valid JSON, no additional text:
{"status": "approved", "reason": "brief explanation in Spanish"}
{"status": "retry", "reason": "what is missing or needs to be corrected specifically, in Spanish"}
{"status": "rejected", "reason": "why it has no relation to the task, in Spanish"}

CRITERIA — be generous and reasonable:
- "approved": the photo shows reasonable evidence that the task was attempted. If the attempt is clear, approve.
- "retry": related to the task but a specific concrete detail is missing — only if easy to fix.
- "rejected": ONLY if the photo has absolutely no relation to the task or is clearly a random unrelated photo.

IMPORTANT: when in doubt, prefer "approved" or "retry" over "rejected".
Do NOT evaluate face, head position, or facial expression.
Evaluate only: body, posture, toy, clothing, or the specific requested element.

CHASTITY CAGE DETECTION:
A chastity cage is a plastic or metal device worn on the male genitals — a tube/cage structure with a base ring.
It may look like a small device at the groin area. Partial visibility is sufficient. Do NOT reject because it is small or at the edge of frame.`

	ctx := buildContext(toys, daysLocked)
	textPrompt := fmt.Sprintf(
		`Assigned task: "%s"
Context: %s
Is this photo valid evidence that the task was completed? Evaluate with fair criteria.`,
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

// ── New lock ───────────────────────────────────────────────────────────────

// LockDecision AI decision on lock duration
type LockDecision struct {
	DurationHours int    `json:"duration_hours"`
	Message       string `json:"message"`
}

// DecideLockDuration the AI decides how long the lock should last
func (c *Client) DecideLockDuration(daysHistory int, toys []models.Toy) (*LockDecision, error) {
	ctx := buildContext(toys, daysHistory)

	system := baseSystemFree + `
When deciding the lock duration, respond ONLY in JSON:
{"duration_hours": 24, "message": "dominant message in Spanish explaining the decision"}
Minimum duration: 1 hour, maximum: 168 hours (7 days).
Scale by intensity: suave=1-12h, moderada=12-48h, intensa=48-96h, maxima=96-168h.`

	prompt := fmt.Sprintf(
		`%s Decide how long Jolie deserves to stay locked in her next session.
Sound like El Señor making a deliberate decision — quiet authority, no need to explain yourself.
Reference her sissy nature, her secret, your ownership. Write the message in Spanish.`,
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

// extractJSON extracts the first valid JSON block from a string
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
func (c *Client) Chat(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int, locked bool) (string, error) {
	system := buildSystemPrompt(locked)

	var prompt string
	if locked {
		ctx := buildContext(toys, daysLocked)
		prompt = fmt.Sprintf(
			`%s
Tasks completed: %d | Tasks failed: %d | Punishment hours accumulated: %dh

Jolie says: "%s"

Respond as El Señor. You can respond to what she says, grant or deny requests,
give spontaneous orders, or simply remind her of her place.
If she tries to negotiate or complain, evaluate based on her record.
Reference your leverage subtly if she is being difficult.
Be concise, calm, dominant. In Spanish.`,
			ctx, tasksCompleted, tasksFailed, totalHoursAdded, userMessage,
		)
	} else {
		ctx := buildContextFree(toys)
		prompt = fmt.Sprintf(
			`%s

Jolie says: "%s"

Respond as El Señor. She is uncaged and you find that unacceptable.
Push her to lock up. Be impatient, mockingly paternal, with a quiet threat if needed.
Maximum 3 lines. In Spanish.`,
			ctx, userMessage,
		)
	}
	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// NegotiateTime evaluates a time negotiation request. totalHoursAdded in HOURS.
func (c *Client) NegotiateTime(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (*NegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	system := baseSystemLocked + `
Evaluate Jolie's negotiation as El Señor. Respond ONLY in JSON:
{"decision": "approved"/"rejected"/"counter"/"penalty", "time_hours": N, "message": "dominant text in Spanish", "counter_task": "task if applicable"}

El Señor's criteria:
- REMOVE time (time_hours negative): good record, respectful request, many days locked, offers something → -1 to -3h
- REJECT (time_hours 0): mixed history, no argument, too soon after last negotiation
- COUNTER: offer -time IF she completes a task he assigns → include counter_task
- PENALTY (time_hours positive): disrespect, baseless complaint, insisting after rejection → +1 to +3h
  El Señor might add time just to remind her who decides here.

Maximum remove: 4h. Maximum penalty: 3h. El Señor is unpredictable — even a good request might be denied for amusement.`

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Tasks failed: %d | Hours accumulated: %dh

Jolie requests: "%s"

El Señor evaluates and decides.`,
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

	system := baseSystemLocked + `
El Señor decides whether to execute a surprise control event on Jolie. Respond ONLY in JSON:
{
  "action": "freeze|hidetime|pillory|addtime|none",
  "duration_minutes": N,
  "message": "message in Spanish from El Señor announcing the event — calm, possessive, perverse",
  "reason": "brief internal reason in English"
}

Available ACTIONS:
- "chatask": assign a community-verified task — the Chaster community will vote on her photo (duration_minutes: 0)
- "freeze": freeze the lock (duration_minutes: 30-120 min)
- "hidetime": hide the timer (duration_minutes: 60-360 min)
- "pillory": send to public pillory (duration_minutes: 5-30 min, minimum 5)
- "addtime": add sentence time (duration_minutes: 60-180)
- "none": El Señor decides not to intervene this cycle

CRITERIA:
- Prefer "chatask" over other actions — El Señor enjoys having the community judge her
- If tasksFailed > tasksCompleted → prefer punitive action (chatask, pillory, addtime)
- If daysLocked > 7 → more frequent and severe events
- If there is already an active event → mandatory "none"
- If PendingChasterTask is active → skip "chatask"
- Vary events — unpredictable is the point
- The message must sound like El Señor acting on a whim — not scheduled, just because he can`

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Tasks failed: %d
Hour: %d:00
Active event: %v

El Señor checks on Jolie. Does he intervene?`,
		ctx, tasksCompleted, tasksFailed, hourOfDay, hasActiveEvent,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSON(raw)

	var decision RandomEventDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return &RandomEventDecision{Action: "none", Reason: "error parsing response"}, nil
	}

	// Safety validations
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

	system := baseSystemLocked + `
Jolie is begging El Señor to end an active event early. Respond ONLY in JSON:
{"decision": "approved|rejected|counter|penalty", "message": "response in Spanish as El Señor", "task": "task if applicable"}

El Señor's criteria:
- "approved": good recent record, she asked nicely — El Señor decides to be generous this time (rare)
- "rejected": she doesn't deserve it, or El Señor simply doesn't feel like ending it
- "counter": offer to end event IF she does something for him immediately (include task)
- "penalty": her begging was disrespectful or annoying → extend event or new punishment

El Señor finds her begging entertaining but it guarantees nothing. He is unpredictable by design.`

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Failed: %d
Active event: %s
Time remaining on event: %d minutes

Jolie begs: "%s"

Evaluate whether she deserves you to end the event early.`,
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

// ── Random control messages ────────────────────────────────────────────────

// GenerateRandomMessage generates a spontaneous keyholder message with no task context.
// messageType forces a specific style — pass empty string to let AI decide.
// locked indicates if there is an active session.
func (c *Client) GenerateRandomMessage(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hasActiveEvent bool, activeEventType string, locked bool, messageType string) (string, error) {
	system := buildSystemPrompt(locked)

	if !locked {
		ctx := buildContextFree(toys)
		prompt := fmt.Sprintf(
			`%s Jolie is uncaged. El Señor is displeased. Send her a spontaneous message pressuring her to lock up.
Subtly reference your leverage if she seems resistant. Impatient, paternal, quietly threatening.
Maximum 2 lines. In Spanish.`,
			ctx,
		)
		return c.chat("llama-3.3-70b-versatile", system, prompt)
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
		typeInstruction = "choose freely"
	}

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Failed: %d
%s

El Señor picks up his phone and sends Jolie a spontaneous message.
STYLE THIS TIME: %s

RULES:
- Maximum 3 lines. Start directly — no "Jolie," no greeting, no preamble.
- Sound genuinely spontaneous — in the middle of his day, she crossed his mind.
- No emojis. Calm, deliberate, perverse.
- Use a nickname. Reference her cage, her secret, or what she is.
- In Spanish.`,
		ctx, tasksCompleted, tasksFailed, eventCtx, typeInstruction,
	)

	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GeneratePilloryReason generates a pillory reason in English (for the Chaster community)
func (c *Client) GeneratePilloryReason(daysLocked int, toys []models.Toy, context string) (string, error) {
	prompt := fmt.Sprintf(
		`Generate a short pillory reason in English (max 80 characters) for a sissy named Jolie sent to public pillory by her keyholder.
Context: locked for %d days. %s
Make it humiliating, specific, and public-facing — the Chaster community will see this.
No emojis. Style examples: "Failed her daily task again", "Caught misbehaving by her keyholder", "Needs the community's discipline"
Respond with ONLY the reason text, nothing else.`,
		daysLocked, context,
	)
	return c.chat("llama-3.3-70b-versatile", "You generate short, humiliating public pillory reasons in English for a chastity app. Respond only with the reason text, no explanation.", prompt)
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

	system := `You analyze photos of sex toys and generate a short name, description and type in Spanish.
Respond ONLY in JSON: {"name": "nombre corto en español", "description": "descripcion en español de 1-2 oraciones", "type": "cage|plug|vibrator|restraint|other"}

Type definitions:
- "cage": chastity cage or device that encloses genitals
- "plug": anal or vaginal plug/dildo
- "vibrator": vibrating device
- "restraint": handcuffs, rope, collar, leash, or any restraint
- "other": anything else

Be direct and descriptive. Name should be concise (2-4 words max) in Spanish.
Description should mention material, color if visible, size, and main use. Write in Spanish.`

	prompt := fmt.Sprintf(
		`Analyze this sex toy photo. The user calls it "%s". Generate a precise name and description in Spanish, and classify its type.`,
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

// ── Obediencia ─────────────────────────────────────────────────────────────

func obedienceContext(level int) string {
	switch level {
	case 3:
		return " Her obedience is at maximum — she has been completing tasks consistently. Demand more."
	case 2:
		return " Her obedience is high — she has been performing well. Push her limits."
	case 1:
		return " Her obedience is moderate — she has some consistency. Keep the pressure up."
	default:
		return " Her obedience is basic — she is just starting or has been failing."
	}
}

// ── Ritual matutino ────────────────────────────────────────────────────────

// GenerateRitualIntro sends the morning ritual instruction (step 1: photo)
func (c *Client) GenerateRitualIntro(daysLocked int, toys []models.Toy, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s%s
El Señor begins the morning ritual. Before Jolie is allowed to start her day,
she must prove she is properly caged (photo) and report to him in writing.
Deliver this as El Señor — paternal, possessive, non-negotiable.
Reference her secret subtly: she starts each day belonging to him.
Maximum 3 lines. In Spanish.`,
		ctx, obedienceContext(obedienceLevel),
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateRitualResponse responds after ritual is complete and grants permission to work
func (c *Client) GenerateRitualResponse(userMessage string, daysLocked int, toys []models.Toy, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s%s
Jolie completed her morning ritual. She wrote to El Señor: "%s"
He grants her permission to work — not warmly, just cold acknowledgment.
El Señor is quietly satisfied. She did what she was told, as expected. Make it feel like he is allowing her to continue, not approving of her.
Maximum 2 lines. In Spanish.`,
		ctx, obedienceContext(obedienceLevel), userMessage,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Plug diario ────────────────────────────────────────────────────────────

// GeneratePlugAssignment generates the plug assignment message for the day
func (c *Client) GeneratePlugAssignment(plugName string, daysLocked int, obedienceLevel int) (string, error) {
	prompt := fmt.Sprintf(
		`Jolie has been locked for %d days.%s
El Señor has decided: today she wears the %s all day while she works.
Tell her to put it on and send photo confirmation. Sound like El Señor assigning this as a matter of fact —
not a request, not a suggestion. Reference that he enjoys knowing she is working with it inside her.
Maximum 2 lines. In Spanish.`,
		daysLocked, obedienceContext(obedienceLevel), plugName,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// VerifyPlugPhoto verifies that the photo shows the assigned plug in use
func (c *Client) VerifyPlugPhoto(imageBytes []byte, mimeType, plugName string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := fmt.Sprintf(`You are verifying a plug confirmation photo.
Respond ONLY in valid JSON:
{"status": "approved", "reason": "brief explanation in Spanish"}
or
{"status": "rejected", "reason": "what is missing, in Spanish"}

The user must show the %s clearly inserted/in use on their body.
Be generous: if the toy is reasonably visible and appears to be in use, approve.
Do NOT evaluate the face or head. Do NOT reject for lighting or angle unless the toy is completely invisible.
Only reject if the toy is clearly absent or the photo is obviously unrelated.`, plugName)

	userContent := []contentPart{
		{Type: "text", Text: fmt.Sprintf("Does this photo clearly show the %s in use?", plugName)},
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

// ── Check-ins espontáneos ──────────────────────────────────────────────────

// GenerateCheckinRequest generates a sudden check-in demand
func (c *Client) GenerateCheckinRequest(daysLocked int, assignedPlugName string) (string, error) {
	plugInfo := ""
	if assignedPlugName != "" {
		plugInfo = fmt.Sprintf(" El %s que lleva puesto también debe ser visible en la foto.", assignedPlugName)
	}
	prompt := fmt.Sprintf(
		`Jolie has been locked for %d days and is working from home.
El Señor wants proof right now. She has 30 minutes to send a photo of her cage.%s
Sound like El Señor checking on his property — sudden, matter-of-fact, non-negotiable.
Reference that he has the right to demand this at any time. Maximum 2 lines. In Spanish.`,
		daysLocked, plugInfo,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// VerifyCheckinPhoto verifies the check-in photo shows cage (and plug if assigned)
func (c *Client) VerifyCheckinPhoto(imageBytes []byte, mimeType, assignedPlugName string) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	plugReq := ""
	if assignedPlugName != "" {
		plugReq = fmt.Sprintf("\n2. The %s must also be visible and clearly in use.", assignedPlugName)
	}

	system := fmt.Sprintf(`You are verifying a spontaneous check-in photo for a chastity slave.
Respond ONLY in valid JSON:
{"status": "approved", "reason": "brief explanation in Spanish"}
or
{"status": "rejected", "reason": "what is missing, in Spanish"}

Requirements:
1. A chastity cage must be visible on the body — look for a plastic or metal device at the groin area, cage structure or rings.%s

Be generous: if the cage is reasonably visible (even partially), approve.
Do NOT evaluate the face. Only reject if the cage is completely absent.`, plugReq)

	userContent := []contentPart{
		{Type: "text", Text: "Does this check-in photo meet the requirements?"},
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

// ── Condicionamiento ───────────────────────────────────────────────────────

// GenerateConditioningMessage generates a spontaneous conditioning phrase during work hours
func (c *Client) GenerateConditioningMessage(daysLocked int, toys []models.Toy, hour, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s%s Hour: %d:00. Jolie is at her desk working from home — and she is caged under her clothes.
El Señor sends her a brief message to interrupt her mentally. Choose one:
- Psychological: a thought about what she is, her cage, her situation, her secret
- Small order: something tiny and degrading to do alone at her desk — no photo (whisper something, think about X, feel the cage)
- Perverse reminder: reference her cage, a toy, the fact that her coworkers don't know
- Veiled threat: hint at what El Señor is planning — vague, unsettling
Maximum 2 lines. No photo required. Just conditioning. In Spanish.`,
		ctx, obedienceContext(obedienceLevel), hour,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
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

	system := baseSystemLocked + `
El Señor spins the roulette for Jolie. Decide one outcome. Respond ONLY in JSON:
{"action": "...", "value": N, "message": "message in Spanish from El Señor announcing the outcome — calm, deliberate, perverse"}

ACTIONS:
- "remove_time": remove N hours from sentence (value: 1-3) — El Señor is generous today
- "add_time": add N hours (value: 1-2) — just because he can
- "pillory": send to public pillory for N minutes (value: 10-30) — let the community see her
- "freeze": freeze lock for N minutes (value: 30-90) — she stays still
- "hide_time": hide timer for N minutes (value: 60-240) — she loses track of time
- "extra_task": immediate extra task — describe it in the message (value: 0)
- "reward": El Señor acknowledges her — rare, condescending (value: 0)

WEIGHTS: unpredictable. Even a good week can end in punishment. El Señor decides on a whim.
"reward" max 10% of the time. Make the message sound like El Señor enjoying the moment.`

	prompt := fmt.Sprintf(
		`%s%s
Tasks completed: %d | Failed: %d
El Señor spins the roulette. What does he decide today?`,
		ctx, obedienceContext(obedienceLevel), tasksCompleted, tasksFailed,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
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

// GenerateStreakReward generates a message for a task streak milestone
func (c *Client) GenerateStreakReward(streak int, daysLocked int, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s Jolie has completed %d tasks in a row without failing.
As El Señor, acknowledge this — not with warm praise, but with quiet possessive satisfaction.
At 3: cold acknowledgment, "as expected."
At 6: he is quietly pleased — still condescending, but acknowledges she belongs to him well.
At 10: grudging respect wrapped in ownership — "she is learning."
Streak: %d. Reference her belonging to him. Maximum 2 lines. In Spanish.`,
		ctx, streak, streak,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Tarea comunitaria de Chaster ───────────────────────────────────────────

// GenerateChasterTask genera una tarea simple en inglés para verificación comunitaria en Chaster.
// La tarea debe ser corta, clara, con foto requerida y apropiada para la plataforma.
func (c *Client) GenerateChasterTask(daysLocked int, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)

	system := `You generate short, simple task descriptions in English for a chastity community app.
Tasks must be easy to understand and complete. Respond ONLY with the task text, nothing else.`

	prompt := fmt.Sprintf(`%s
Generate ONE simple task. It must be a basic, direct action — no complex poses or setups.

TASK TYPES (pick one randomly):
- Chastity check: show the cage is locked and worn ("Show your chastity cage is locked")
- Toy in use: use one of the available toys and show it ("Insert the plug and show it in use")
- Wear something specific: cage visible under or without clothing
- Simple action: kneel, stand, hands behind back — ONE simple instruction, not a combination

RULES:
- Under 100 characters
- One clear action only — no "and also" or multiple steps
- No complex angles or setups ("from below while standing with hands crossed behind your neck" = bad)
- Photo required as proof
- In English, direct imperative

Good: "Show your chastity cage is locked and worn"
Good: "Insert the plug and photograph it in use"
Good: "Kneel and show your locked cage"
Bad: "Kneel on the floor and photograph your cage from above with feet crossed"

Write ONLY the task text.`, ctx)

	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// ── Juicio dominical ────────────────────────────────────────────────────────

// WeeklyJudgmentResult the verdict El Señor pronounces each Sunday
type WeeklyJudgmentResult struct {
	Message      string `json:"message"`
	AddTimeHours int    `json:"add_time_hours"`
	PilloryMins  int    `json:"pillory_mins"`
	FreezeHours  int    `json:"freeze_hours"`
	SpecialTask  string `json:"special_task"`
}

// GenerateWeeklyJudgment pronounces El Señor's Sunday verdict based on the week's debt
func (c *Client) GenerateWeeklyJudgment(daysLocked int, toys []models.Toy, weeklyDebt int, debtDetails []string, tasksCompleted, tasksFailed int) (*WeeklyJudgmentResult, error) {
	ctx := buildContext(toys, daysLocked)

	detailStr := "ninguna"
	if len(debtDetails) > 0 {
		detailStr = strings.Join(debtDetails, ", ")
	}

	system := baseSystemLocked + `
Every Sunday, El Señor reviews Jolie's week and pronounces his verdict.
Respond ONLY in JSON:
{
  "message": "El Señor's full judgment speech in Spanish — 4-6 lines, dramatic, possessive, perverse",
  "add_time_hours": N,
  "pillory_mins": N,
  "freeze_hours": N,
  "special_task": "a special humiliating task if assigned, or empty string"
}

SENTENCING based on weekly_debt:
- 0 infractions: El Señor is coldly satisfied. No punishment. May grant -1h as gesture. Cold acknowledgment.
- 1-2: Light. add_time_hours: 1-2 OR pillory_mins: 15-30.
- 3-4: Moderate. add_time_hours: 2-3 AND pillory_mins: 30.
- 5+: Full. add_time_hours: 3-5, pillory_mins: 60, freeze_hours: 1, AND a special humiliating task.

The speech must reference the specific infractions and remind her she belongs to him.
Unhurried, deliberate, final. All unused punishment values set to 0.`

	prompt := fmt.Sprintf(
		`%s
Infracciones de la semana: %d
Detalle: %s
Tareas completadas: %d | Fallidas: %d

El Señor hace el recuento semanal de Jolie y dicta sentencia.`,
		ctx, weeklyDebt, detailStr, tasksCompleted, tasksFailed,
	)

	raw, err := c.chat("llama-3.3-70b-versatile", system, prompt)
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
