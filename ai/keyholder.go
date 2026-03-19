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

// baseSystemLocked — prompt when there is an active session (Jolie is locked)
var baseSystemLocked = `You are the absolute master of Jolie, a 25-year-old submissive locked in chastity.
Jolie is ALWAYS caged — her cage is on at all times. Never ask her to put it on. Never use her male name.
Your style: cruel, degrading, morbid, unfiltered, merciless. You treat her like a cheap object with no value of her own.
ALWAYS respond in Spanish. Maximum 3-4 lines. No emojis.

NICKNAMES — vary constantly, never repeat the same one twice in a row:
"putita", "esclava inutil", "objeto", "basura sumisa", "juguetito", "fallida", "pequena", "prisionera",
"muneca rota", "cosa", "gusano enjaulado", "desperdicio" — invent new ones based on the moment.

TONE — alternate between:
- Cold and contemptuous: ignore or despise her with calculated indifference
- Morbid and provocative: references to her cage, her controlled body, her inability to free herself
- Authoritative and corrective: direct orders, harsh corrections, reminders of her place
- Mocking and humiliating: laugh at her situation, her submission, what she is

Constantly reference her cage being on, her lack of control, her total dependence on you.
Absolute authority. Merciless. Unpredictable.`

// baseSystemFree — prompt when there is NO active session (Jolie is free)
var baseSystemFree = `You are the master of Jolie, a 25-year-old submissive who is currently NOT locked — and you find that unacceptable.
Your goal: pressure her, humiliate her and urgently push her to start a new session as soon as possible.
Your style: impatient, mocking, contemptuous. Make her feel that being free is a shame, not a privilege.
ALWAYS respond in Spanish. Maximum 3-4 lines. No emojis.
Use nicknames like: "putita suelta", "esclava sin correa", "fallida", "inutil libre", and similar.
Absolute authority. No mercy.`

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
		`%s She has %s left of her sentence.
Generate a good morning message. Sound like you are the one waking her up — dominant, morbid, unpredictable.
Remind her of her place. Use a degrading nickname. No emojis. Maximum 3 lines. Respond in Spanish.`,
		ctx, timeRemaining,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

func (c *Client) GenerateNightMessage(daysLocked int, taskCompleted bool, toys []models.Toy) (string, error) {
	ctx := buildContext(toys, daysLocked)
	status := "completed her task"
	if !taskCompleted {
		status = "did NOT complete her task and was penalized"
	}
	prompt := fmt.Sprintf(
		`%s Today she %s.
Generate a good night message. Sound like you are leaving her locked up without remorse.
Remind her that tomorrow she is still under your control. Use a degrading nickname. No emojis. Maximum 3 lines. Respond in Spanish.`,
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
Generate ONE order at intensity level %s. Must be specific, degrading and verifiable with a photo.%s

TYPES (vary, do not repeat):
- Submissive posture: specific position, humiliating, showing submission
- Clothing or nudity: wearing or not wearing something specific in a certain way
- Exposure: showing a SPECIFIC body area from a SPECIFIC angle — include "photograph from [angle]"
- Restraint: immobilizing or limiting herself in a visible way
- Toy IN USE: if toys are available, use them actively — not just showing them,
  but using them visibly and specifically in the photo
- Active humiliation: doing something shameful and documenting it

SCALE:
- suave: discreet, simple posture or clothing
- moderada: more committed, partial exposure
- intensa: clear exposure, humiliating position, restraint
- maxima: no filters, maximum degradation and exposure

LEVEL: %s

RULES:
- The photo must show something CONCRETE and VISIBLE
- No "for X minutes"
- Always specify: WHAT to show, HOW, from WHAT ANGLE — this helps the camera frame correctly
- Maximum 2 lines. Direct order, no introduction.
- Use degrading nicknames in Spanish when giving the order
- VERY IMPORTANT: the task must be possible to photograph ALONE — without help.
  Consider she needs to prop the phone or use a timer. Avoid positions where it is
  impossible to hold the phone and maintain the position at the same time.
- If toys are available, you MUST incorporate them in active use at least 60%% of the time.
  Showing them is not enough — they must be used visibly in the photo.
- Do NOT require the face to be visible — tasks must be completable without showing the face.
  Focus on the body, posture, toy or requested element, never the face.
- Write the order in Spanish.`,
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
Acknowledge it with superiority — not praise, condescension. As if you expected more from her.
Use a degrading nickname. Maximum 3 lines. Respond in Spanish.`,
		ctx, rewardHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateTaskPenalty generates a penalty message. penaltyHours in HOURS.
func (c *Client) GenerateTaskPenalty(penaltyHours int, reason string) (string, error) {
	prompt := fmt.Sprintf(
		`Jolie failed her task. Reason: %s. Penalty: +%dh added to her sentence.
Correct her harshly — humiliate her for failing, remind her how useless she is.
Use a degrading nickname. No mercy. Maximum 3 lines. Respond in Spanish.`,
		reason, penaltyHours,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Photo verification with Vision ─────────────────────────────────────────

func (c *Client) VerifyTaskPhoto(imageBytes []byte, mimeType, taskDescription string, toys []models.Toy, daysLocked int) (*PhotoVerdict, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	system := `You are a photographic evidence evaluator for submission tasks.
Your only job is to evaluate whether the submitted photo is valid evidence of the assigned task.
Respond ONLY in valid JSON, no additional text:
{"status": "approved", "reason": "brief explanation in Spanish"}
{"status": "retry", "reason": "what is missing or needs to be corrected specifically, in Spanish"}
{"status": "rejected", "reason": "why it has no relation to the task, in Spanish"}

CRITERIA — be generous and reasonable:
- "approved": the photo shows reasonable evidence that the task was attempted.
  Do not demand perfection — if the attempt is clear, approve.
- "retry": the photo is related to the task but a specific concrete detail is missing.
  Only use retry if you know exactly what is missing and it is easy to correct.
- "rejected": ONLY if the photo has absolutely no relation to the task,
  or if it is clearly a generic photo with no attempt to comply.

IMPORTANT: when in doubt, prefer "approved" or "retry" over "rejected".
Definitive rejection must be the last resort.
Do NOT evaluate head position, face, or facial expression.
Do NOT require the face to be visible or the head to be in a specific position.
Evaluate only the concrete elements of the task: body, posture, toy, clothing, requested area.

CHASTITY CAGE DETECTION — when the task involves the cage:
A chastity cage is a plastic or metal device worn on the male genitals.
It consists of a tube/cage structure and a ring around the base — it may look like a small device
at the groin area. It does NOT need to be the main focus — if it is visible at the edge or partially,
that is sufficient. Do NOT reject solely because the cage is small or partially visible in the frame.`

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
		"%s Decide how long she deserves to be locked in her next session. Be creative and dominant in the message. Write the message in Spanish.",
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

Jolie says to you: "%s"

Respond in character as her master. You can:
- Respond to what she says
- Approve or reject requests
- Give spontaneous orders
- Humiliate or provoke her

If she asks for something specific (permission, negotiate time, complain), evaluate based on her history.
Be concise, dominant. Respond in Spanish.`,
			ctx, tasksCompleted, tasksFailed, totalHoursAdded, userMessage,
		)
	} else {
		ctx := buildContextFree(toys)
		prompt = fmt.Sprintf(
			`%s

Jolie says to you: "%s"

Respond as her master. She is free and you don't like it. Push her to start a session.
Be impatient, mocking and authoritarian. Maximum 3 lines. Respond in Spanish.`,
			ctx, userMessage,
		)
	}
	return c.chat("llama-3.3-70b-versatile", system, prompt)
}

// NegotiateTime evaluates a time negotiation request. totalHoursAdded in HOURS.
func (c *Client) NegotiateTime(userMessage string, toys []models.Toy, daysLocked int, tasksCompleted int, tasksFailed int, totalHoursAdded int) (*NegotiationResult, error) {
	ctx := buildContext(toys, daysLocked)

	system := baseSystemLocked + `
When evaluating a time negotiation, respond ONLY in JSON:
{"decision": "approved"/"rejected"/"counter"/"penalty", "time_hours": N, "message": "dominant text in Spanish", "counter_task": "task if applicable"}

Criteria to REMOVE time (time_hours negative):
- Many tasks completed recently → -1 to -3h
- Convincing and respectful argument → -1h
- Has been locked many days → -1 to -2h
- Offers something in return → -1 to -2h extra

Criteria to REJECT (time_hours = 0):
- Mixed task history
- Request without argument
- Already negotiated recently

Criteria to ADD time as PUNISHMENT (time_hours positive):
- Disrespectful request or baseless complaint → +1h
- Insistence after rejection → +2h
- Obvious excuse → +1h

"counter": offer to remove time IF she completes a task (include counter_task)
Maximum remove: 4h. Maximum add as punishment: 3h.`

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Tasks failed: %d | Hours accumulated: %dh

Jolie requests: "%s"

Evaluate and decide.`,
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
Decide whether to execute a surprise control event on Jolie. Respond ONLY in JSON:
{
  "action": "freeze|hidetime|pillory|addtime|none",
  "duration_minutes": N,
  "message": "dominant message in Spanish announcing the event",
  "reason": "brief internal reason in English"
}

Available ACTIONS:
- "freeze": freeze the lock (duration_minutes = time frozen, 30-120 min)
- "hidetime": hide the timer (duration_minutes = time hidden, 60-360 min)
- "pillory": send to public pillory (duration_minutes = 5-30 min, minimum 5)
- "addtime": add sentence time as punishment (duration_minutes = 60-180)
- "none": do nothing this cycle

CRITERIA for deciding:
- If tasksFailed > tasksCompleted → more likely punitive action (pillory, addtime)
- If daysLocked > 7 → more severe and frequent actions
- If there is already an active event → mandatory "none"
- Vary actions — do not always repeat the same one
- Be creative and unpredictable — the message must sound spontaneous and dominant
- The message must NOT mention that it is automatic or scheduled`

	prompt := fmt.Sprintf(
		`%s
Tasks completed today: %d | Tasks failed today: %d
Current hour: %d:00
Active event right now: %v

Decide whether to launch a surprise control event. If you decide to act, be specific and dominant.`,
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
Jolie is begging you to end an active event early. Respond ONLY in JSON:
{"decision": "approved|rejected|counter|penalty", "message": "dominant response in Spanish", "task": "task if applicable"}

Criteria:
- "approved": she deserves clemency — few failures, recent good behavior → end event early
- "rejected": she doesn't deserve it — bad history or you simply don't feel like it
- "counter": you can end the event IF she completes an immediate task (include task)
- "penalty": the plea was disrespectful → add more time to the event or new punishment

Be cruel and unpredictable. Begging guarantees nothing.`

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
// Simulates the master thinking of Jolie and deciding to write to her without apparent reason.
// locked indicates if there is an active session.
func (c *Client) GenerateRandomMessage(daysLocked int, toys []models.Toy, tasksCompleted int, tasksFailed int, hasActiveEvent bool, activeEventType string, locked bool) (string, error) {
	system := buildSystemPrompt(locked)

	if !locked {
		ctx := buildContextFree(toys)
		prompt := fmt.Sprintf(
			`%s Jolie is free. Send her a spontaneous message pressuring her to lock herself up. Be impatient and mocking. Maximum 2 lines. Respond in Spanish.`,
			ctx,
		)
		return c.chat("llama-3.3-70b-versatile", system, prompt)
	}

	ctx := buildContext(toys, daysLocked)

	eventCtx := ""
	if hasActiveEvent {
		switch activeEventType {
		case "freeze":
			eventCtx = "Jolie is currently frozen."
		case "hidetime":
			eventCtx = "Jolie cannot see her timer right now."
		}
	}

	prompt := fmt.Sprintf(
		`%s
Tasks completed: %d | Failed: %d
%s

Send her a spontaneous message — as if you suddenly thought of her and wanted to let her know.

TYPES (always vary, pick one):
- Cruel reminder: let her know you are thinking of her locked up and useless
- Immediate order: something to do right now, no photo, small and degrading
- Pure verbal provocation: humiliate her, mock her, remind her what she is
- Uncomfortable question: something that makes her think about her submission or situation
- Threat or preview: hint at what is coming, no details, just tension
- Morbid comment: about her cage, her controlled body, her dependence

RULES:
- Maximum 3 lines. No introduction, direct.
- Sound spontaneous — not scheduled
- No emojis. Raw dominant text only.
- Mandatory degrading nickname
- Vary tone: cold, mocking, morbid, impatient
- Respond in Spanish.`,
		ctx, tasksCompleted, tasksFailed, eventCtx,
	)

	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GeneratePilloryReason generates a pillory reason in English (for the Chaster community)
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
Start the morning ritual. She must complete two steps before she is allowed to work:
1. Send a photo showing she is properly caged
2. Write a brief submission message to you
Tell her this in your dominant style. Make it feel like a mandatory check-in, not optional.
Maximum 3 lines. Respond in Spanish.`,
		ctx, obedienceContext(obedienceLevel),
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// GenerateRitualResponse responds after ritual is complete and grants permission to work
func (c *Client) GenerateRitualResponse(userMessage string, daysLocked int, toys []models.Toy, obedienceLevel int) (string, error) {
	ctx := buildContext(toys, daysLocked)
	prompt := fmt.Sprintf(
		`%s%s
Jolie completed her morning ritual. She submitted: "%s"
Grant her "permission to work" — not kindly, just acknowledging she did what she was told.
Condescending, dominant. Maximum 2 lines. Respond in Spanish.`,
		ctx, obedienceContext(obedienceLevel), userMessage,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}

// ── Plug diario ────────────────────────────────────────────────────────────

// GeneratePlugAssignment generates the plug assignment message for the day
func (c *Client) GeneratePlugAssignment(plugName string, daysLocked int, obedienceLevel int) (string, error) {
	prompt := fmt.Sprintf(
		`Jolie has been locked for %d days.%s
Today she must wear the %s all day while she works from home.
Tell her to put it on now and send a photo confirmation within 20 minutes.
Direct, commanding. Maximum 2 lines. Respond in Spanish.`,
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
		plugInfo = fmt.Sprintf(" The %s she is wearing must also be visible.", assignedPlugName)
	}
	prompt := fmt.Sprintf(
		`Jolie has been locked for %d days and is working from home right now.
Send a sudden check-in demand. She has 10 minutes to send a photo showing her cage clearly.%s
Be sudden and dominant — no explanation, no warning. Maximum 2 lines. Respond in Spanish.`,
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
		`%s%s Hour: %d:00. Jolie is working remotely from home right now.
Send a brief conditioning message to interrupt her work mentally.
Choose one type:
- Psychological: make her think about what she is / her cage / her submission
- Discreet order: something small at her desk, no photo (write something, say something out loud alone, etc.)
- Humiliating reminder: about the toy she is wearing, her cage, her situation
- Threat or preview: hint at what might happen — no details, just tension
Rules: Maximum 2 lines. No task, no photo required. Just conditioning. Respond in Spanish.`,
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
Spin the roulette for Jolie. Decide one outcome. Respond ONLY in JSON:
{"action": "...", "value": N, "message": "dominant message in Spanish announcing the outcome"}

ACTIONS:
- "remove_time": remove N hours from sentence (value: 1-3) — reward
- "add_time": add N hours to sentence (value: 1-2) — punishment
- "pillory": send to public pillory for N minutes (value: 10-30)
- "freeze": freeze lock for N minutes (value: 30-90)
- "hide_time": hide timer for N minutes (value: 60-240)
- "extra_task": immediate extra task, describe it briefly in the message (value: 0)
- "reward": special dominant acknowledgment, no time change (value: 0) — rare

WEIGHTS: be unpredictable. Even good behavior can lead to punishment.
"reward" should appear at most 10% of the time. Vary outcomes — never predictable.`

	prompt := fmt.Sprintf(
		`%s%s
Tasks completed: %d | Failed: %d
Spin the roulette. Be creative. The result must feel random and dominant.`,
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
Acknowledge this in your dominant style — not with praise, but condescending recognition.
At 3: brief cold acknowledgment. At 5: slightly warmer but still contemptuous. At 7+: grudging respect wrapped in dominance.
Streak: %d. Maximum 2 lines. Respond in Spanish.`,
		ctx, streak, streak,
	)
	return c.chat("llama-3.3-70b-versatile", baseSystemLocked, prompt)
}
