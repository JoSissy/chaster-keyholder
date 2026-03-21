package models

import "time"

// ChasterLock representa una sesión de castidad activa
type ChasterLock struct {
	ID              string     `json:"_id"`
	Status          string     `json:"status"`
	StartDate       time.Time  `json:"-"`
	EndDate         *time.Time `json:"endDate,omitempty"`
	TotalDuration   int64      `json:"totalDuration"`
	Title           string     `json:"title"`
	Combination     string     `json:"combination,omitempty"`
	Frozen          bool       `json:"isFrozen"`
	IsReadyToUnlock bool       `json:"isReadyToUnlock"`
}

// Task representa una tarea diaria — se persiste en DB
type Task struct {
	ID            string    `json:"id"`
	LockID        string    `json:"lock_id"`
	Description   string    `json:"description"`
	PhotoURL      string    `json:"photo_url,omitempty"`
	AssignedAt    time.Time `json:"assigned_at"`
	DueAt         time.Time `json:"due_at"`
	Completed     bool      `json:"completed"`
	Failed        bool      `json:"failed"`
	PenaltyHours  int       `json:"penalty_hours"`
	RewardHours   int       `json:"reward_hours"`
	AwaitingPhoto bool      `json:"awaiting_photo"`
}

// Toy representa un juguete — se persiste en DB
type Toy struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	PhotoURL      string    `json:"photo_url"`
	PhotoPublicID string    `json:"photo_public_id,omitempty"`
	Type          string    `json:"type"`   // "cage", "plug", "vibrator", "restraint", "other"
	InUse         bool      `json:"in_use"` // true si está puesto ahora mismo
	AddedAt       time.Time `json:"added_at"`
}

// ActiveEvent representa un evento random activo con auto-reversión
type ActiveEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "freeze" | "hidetime"
	ExpiresAt time.Time `json:"expires_at"`
}

// IntensityLevel nivel de intensidad según días encerrada
type IntensityLevel int

const (
	IntensityLight    IntensityLevel = 1
	IntensityModerate IntensityLevel = 2
	IntensityIntense  IntensityLevel = 3
	IntensityMaximum  IntensityLevel = 4
)

func (i IntensityLevel) String() string {
	switch i {
	case IntensityLight:
		return "suave"
	case IntensityModerate:
		return "moderada"
	case IntensityIntense:
		return "intensa"
	case IntensityMaximum:
		return "máxima"
	}
	return "suave"
}

func GetIntensity(daysLocked int) IntensityLevel {
	switch {
	case daysLocked >= 15:
		return IntensityMaximum
	case daysLocked >= 8:
		return IntensityIntense
	case daysLocked >= 4:
		return IntensityModerate
	default:
		return IntensityLight
	}
}

// ObedienceTitle devuelve el título según los puntos de obediencia.
func ObedienceTitle(points int) string {
	switch {
	case points >= 21:
		return "esclava perfecta de Papi"
	case points >= 15:
		return "puta obediente de Papi"
	case points >= 9:
		return "culo en formación"
	case points >= 4:
		return "sissy sin entrenar"
	default:
		return "maricona desobediente"
	}
}

// GetObedienceLevel devuelve 0-4 según los puntos de obediencia.
func GetObedienceLevelFromPoints(points int) int {
	switch {
	case points >= 21:
		return 4
	case points >= 15:
		return 3
	case points >= 9:
		return 2
	case points >= 4:
		return 1
	default:
		return 0
	}
}

// AppState es el estado central del bot. Se persiste en dos lugares:
//   - state.json (primario, escritura atómica via archivo temporal)
//   - tabla session_state en SQLite (respaldo de contadores críticos)
//
// Al arrancar, loadState() lee state.json; si los contadores están en cero,
// los restaura desde la DB. Si state.json no existe o está corrupto, carga todo desde DB.
//
// IMPORTANTE: pendingActions y pendingToyHint NO se persisten — son estado transitorio
// de UI que se reconstruye en NewBot() a partir de los flags del estado.
type AppState struct {
	// Tarea activa del día. nil si no hay tarea asignada todavía.
	CurrentTask *Task `json:"current_task,omitempty"`

	// Tiempo total añadido/quitado a la condena durante esta sesión (en horas).
	TotalTimeAddedHours   int `json:"total_time_added_hours"`
	TotalTimeRemovedHours int `json:"total_time_removed_hours"`

	// Cache local de días encerrada — actualizado por daysLocked() cada 5 minutos.
	DaysLocked int `json:"days_locked"`

	// Lista de juguetes registrados. Siempre cargada desde DB en NewBot().
	Toys []Toy `json:"toys"`

	// true mientras se espera la foto de confirmación del lock inicial (/newlock).
	AwaitingLockPhoto bool `json:"awaiting_lock_photo"`

	// ID del lock activo en Chaster (string _id de la API).
	CurrentLockID string `json:"current_lock_id"`

	// Duración manual de /newlock [duración]. Si > 0, se usa en lugar de dejar
	// que la IA decida. Se limpia a 0 inmediatamente después de crear el lock.
	// Formato: segundos. Ejemplo: "/newlock 2 días" → 172800.
	ManualDurationSeconds int `json:"manual_duration_seconds"`

	// Contadores de tareas completadas y falladas en esta sesión.
	TasksCompleted int `json:"tasks_completed"`
	TasksFailed    int `json:"tasks_failed"`

	// Evento random activo (freeze o hidetime). nil si no hay evento activo.
	ActiveEvent *ActiveEvent `json:"active_event,omitempty"`

	// TasksStreak es un ACUMULADOR DE PUNTOS DE OBEDIENCIA, no un streak tradicional.
	// Sube: +1 por tarea completada (o +2 si lleva 8+ días encerrada),
	//       +3 bonus cada 7 días consecutivos de tareas,
	//       +1 cada 2 confirmaciones de plug (vía PlugBonusAccum).
	// Baja: -3 al fallar una tarea, -1 por insistencia penalizada.
	// Nunca resetea a 0 completamente — es un total histórico que determina
	// el título de obediencia (ObedienceTitle) y la probabilidad de orgasmo (rollOrgasmOutcome).
	TasksStreak int `json:"tasks_streak"`

	// ── Ritual matutino ──────────────────────────────────────────────────────
	// Cada día a las 8:30 AM se lanza el ritual. Flujo: foto de jaula → mensaje escrito.
	LastRitualDate string `json:"last_ritual_date"` // "2006-01-02" COT — evita repetir el ritual el mismo día
	RitualStep     int    `json:"ritual_step"`       // 0=sin iniciar/completado, 1=esperando foto, 2=esperando mensaje

	// ── Fechas del lock activo ───────────────────────────────────────────────
	// Sincronizadas desde la API de Chaster al arrancar el bot.
	LockEndDate   *time.Time `json:"lock_end_date,omitempty"`
	LockStartDate *time.Time `json:"lock_start_date,omitempty"`

	// ── Outfit diario ────────────────────────────────────────────────────────
	// Asignado como parte del ritual matutino (scheduler). La foto se verifica via IA.
	DailyOutfitDesc     string `json:"daily_outfit_desc"`
	DailyOutfitDate     string `json:"daily_outfit_date"` // "2006-01-02" COT
	DailyPoseDesc       string `json:"daily_pose_desc"`
	DailyOutfitPhotoURL string `json:"daily_outfit_photo_url"` // URL en Cloudinary tras aprobación
	OutfitConfirmed     bool   `json:"outfit_confirmed"`
	DailyOutfitComment  string `json:"daily_outfit_comment"` // comentario de Papi al aprobar la foto

	// ── Plug diario ──────────────────────────────────────────────────────────
	// A las 8:45 AM se asigna un plug aleatorio de los juguetes tipo "plug".
	// La foto es verificada por IA (VerifyPlugPhoto).
	AssignedPlugID   string `json:"assigned_plug_id"`   // ID del toy asignado
	AssignedPlugDate string `json:"assigned_plug_date"` // "2006-01-02" COT
	PlugConfirmed    bool   `json:"plug_confirmed"`
	PlugReminderDate string `json:"plug_reminder_date"` // "2006-01-02" COT — evita recordatorios duplicados por día

	// ── Check-ins espontáneos ────────────────────────────────────────────────
	// Disparados por el scheduler (11:00 y 15:00 COT, aleatoriamente).
	// Jolie tiene 30 minutos para mandar una foto de la jaula.
	// La foto se sube directamente a Chaster (verificación comunitaria, no IA).
	// CheckinVerificationCode es un código de 6 dígitos que debe ser visible en la foto
	// para que la comunidad de Chaster confirme que es en tiempo real.
	PendingCheckin          bool       `json:"pending_checkin"`
	CheckinExpiresAt        *time.Time `json:"checkin_expires_at,omitempty"`
	CheckinReminderSent     bool       `json:"checkin_reminder_sent"` // true si ya se mandó el aviso de "5 minutos"
	CurrentCheckinID        string     `json:"current_checkin_id,omitempty"`
	CheckinVerificationCode string     `json:"checkin_verification_code,omitempty"`

	// ── Ruleta diaria ────────────────────────────────────────────────────────
	// Disponible una vez por día a las 18:00. Resultados variables (tiempo, evento, etc.).
	LastRuletaDate string `json:"last_ruleta_date"` // "2006-01-02" COT

	// ── Cooldown de orgasmo ──────────────────────────────────────────────────
	// Evita que Jolie pida permiso repetidamente. El cooldown depende del resultado:
	// granted_cum=24h, denied=6h.
	LastOrgasmRequestAt *time.Time `json:"last_orgasm_request_at,omitempty"`
	LastOrgasmOutcome   string     `json:"last_orgasm_outcome,omitempty"` // "denied" | "granted_cum"

	// Cuántas veces ha insistido durante el cooldown activo.
	// Al llegar a 3 se hace un roll especial (bien/medio/mal). Se resetea tras el roll.
	CooldownInsistCount int `json:"cooldown_insist_count,omitempty"`

	// Cuando Papi concede un orgasmo real (granted_cum) pero Jolie aún no lo ha
	// reportado con /came. Ventana de 1 hora. Se limpia al recibir el reporte o al expirar.
	GrantedCumPendingAt *time.Time `json:"granted_cum_pending_at,omitempty"`

	// Cuando Papi ordena una sesión con juguetes (granted_toys). Ventana de 1 hora.
	// Se limpia al confirmar o al expirar.
	GrantedToysPendingAt *time.Time `json:"granted_toys_pending_at,omitempty"`

	// Instrucciones específicas que Papi dio al conceder el permiso (granted_cum).
	// Se pasa a GenerateCameResponse para que Papi las referencie al reaccionar.
	GrantedCondition string `json:"granted_condition,omitempty"`

	// Última sesión de juguetes completada. Se usa para bloquear sesiones
	// repetidas en menos de 4 horas.
	LastToySessionAt *time.Time `json:"last_toy_session_at,omitempty"`

	// ── Obediencia avanzada ──────────────────────────────────────────────────

	// Días seguidos con al menos una tarea completada. Se incrementa en HandlePhoto
	// y se evalúa en CheckObedienceDecay (si pasan 2 días sin tarea, resetea a 0).
	// Cada 7 días consecutivos otorga +3 puntos de obediencia (TasksStreak).
	ConsecutiveDays int `json:"consecutive_days"`

	// Acumulador de confirmaciones de plug. Cada 2 confirmaciones exitosas suma
	// +1 a TasksStreak y resetea a 0. Permite ganar obediencia via plug además
	// de via tareas, lo que facilita llegar a mejores probabilidades de orgasmo.
	PlugBonusAccum int `json:"plug_bonus_accum"`

	LastTaskCompletedDate string `json:"last_task_completed_date"` // "2006-01-02" COT

	// ── Deuda semanal ────────────────────────────────────────────────────────
	// WeeklyDebt es un contador simple de infracciones acumuladas desde el último
	// juicio dominical (domingo 21:00 COT). Cada infracción suma 1.
	// WeeklyDebtDetails guarda una descripción de cada infracción (para el prompt del juicio).
	// Se resetean a 0 después del juicio semanal (SendWeeklyJudgment).
	// Infracciones que suman: tarea fallida, check-in ignorado, plug no confirmado,
	// ritual ignorado, orgasmo sin permiso, tarea comunitaria rechazada, insistencia castigada.
	WeeklyDebt        int      `json:"weekly_debt"`
	WeeklyDebtDetails []string `json:"weekly_debt_details,omitempty"`
	LastJudgmentDate  string   `json:"last_judgment_date"` // "2006-01-02" COT

	// ── Tarea comunitaria de Chaster ─────────────────────────────────────────
	// Asignada via Extensions API. La foto se sube a Chaster y es verificada
	// por la comunidad (no por IA local). ChasterTaskSessionID es el ID de sesión
	// de la extensión, necesario para SubmitVerificationPicture.
	PendingChasterTask    string     `json:"pending_chaster_task,omitempty"`     // descripción de la tarea, vacío si no hay tarea pendiente
	ChasterTaskSessionID  string     `json:"chaster_task_session_id,omitempty"`  // ID de sesión de extensión para el submit
	ChasterTaskLockID     string     `json:"chaster_task_lock_id,omitempty"`     // lock al que pertenece
	ChasterTaskAssignedAt *time.Time `json:"chaster_task_assigned_at,omitempty"` // cuándo se asignó
	ChasterTaskDBID       string     `json:"chaster_task_db_id,omitempty"`       // ID en la tabla chaster_tasks de la DB local
}

// ChatMessage un mensaje de la historia de conversación con Papi
type ChatMessage struct {
	Role    string // "user" | "assistant"
	Content string
}

// ContractRule una regla del contrato activo verificable por chat
type ContractRule struct {
	ID         string
	LockID     string
	RuleText   string
	Punishment string // "add_time" | "pillory" | "freeze"
	Hours      int
	Minutes    int
}

// GetObedienceLevel devuelve el nivel de obediencia (0-3) según el streak actual.
// OBSOLETO — esta función usa una escala distinta (0-3, thresholds 3/6/10) a la
// usada en todo el resto del código (GetObedienceLevelFromPoints, escala 0-4).
// No se llama desde ningún lugar. Conservada por si se quiere reutilizar.
func GetObedienceLevel(tasksStreak int) int {
	switch {
	case tasksStreak >= 10:
		return 3
	case tasksStreak >= 6:
		return 2
	case tasksStreak >= 3:
		return 1
	default:
		return 0
	}
}

// ObedienceLevelString devuelve el nombre del nivel de obediencia
func ObedienceLevelString(level int) string {
	switch level {
	case 3:
		return "máximo"
	case 2:
		return "intenso"
	case 1:
		return "moderado"
	default:
		return "básico"
	}
}
