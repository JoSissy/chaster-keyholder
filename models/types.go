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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PhotoURL    string    `json:"photo_url"`
	Type        string    `json:"type"`   // "cage", "plug", "vibrator", "restraint", "other"
	InUse       bool      `json:"in_use"` // true si está puesto ahora mismo
	AddedAt     time.Time `json:"added_at"`
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

// AppState estado en memoria — se sincroniza con la DB periódicamente
type AppState struct {
	CurrentTask           *Task        `json:"current_task,omitempty"`
	TotalTimeAddedHours   int          `json:"total_time_added_hours"`
	TotalTimeRemovedHours int          `json:"total_time_removed_hours"`
	DaysLocked            int          `json:"days_locked"`
	Toys                  []Toy        `json:"toys"`
	AwaitingLockPhoto     bool         `json:"awaiting_lock_photo"`
	CurrentLockID         string       `json:"current_lock_id"`
	ManualDurationSeconds int          `json:"manual_duration_seconds"`
	TasksCompleted        int          `json:"tasks_completed"`
	TasksFailed           int          `json:"tasks_failed"`
	ActiveEvent           *ActiveEvent `json:"active_event,omitempty"`
	TasksStreak           int          `json:"tasks_streak"`

	// Ritual matutino
	LastRitualDate string `json:"last_ritual_date"` // "2006-01-02" COT
	RitualStep     int    `json:"ritual_step"`       // 0=none/done, 1=awaiting photo, 2=awaiting message

	// Fechas del lock activo (sincronizadas desde Chaster al arrancar)
	LockEndDate   *time.Time `json:"lock_end_date,omitempty"`
	LockStartDate *time.Time `json:"lock_start_date,omitempty"`

	// Outfit diario
	DailyOutfitDesc     string `json:"daily_outfit_desc"`
	DailyOutfitDate     string `json:"daily_outfit_date"` // "2006-01-02" COT
	DailyPoseDesc       string `json:"daily_pose_desc"`
	DailyOutfitPhotoURL string `json:"daily_outfit_photo_url"`
	OutfitConfirmed     bool   `json:"outfit_confirmed"`
	DailyOutfitComment  string `json:"daily_outfit_comment"` // comentario de Papi al aprobar

	// Control de plug diario
	AssignedPlugID   string `json:"assigned_plug_id"`
	AssignedPlugDate string `json:"assigned_plug_date"` // "2006-01-02" COT
	PlugConfirmed    bool   `json:"plug_confirmed"`

	// Check-ins espontáneos
	PendingCheckin      bool       `json:"pending_checkin"`
	CheckinExpiresAt    *time.Time `json:"checkin_expires_at,omitempty"`
	CheckinReminderSent bool       `json:"checkin_reminder_sent"`

	// Ruleta
	LastRuletaDate string `json:"last_ruleta_date"` // "2006-01-02" COT

	// Deuda semanal
	WeeklyDebt        int      `json:"weekly_debt"`
	WeeklyDebtDetails []string `json:"weekly_debt_details,omitempty"`
	LastJudgmentDate  string   `json:"last_judgment_date"` // "2006-01-02" COT

	// Tarea comunitaria de Chaster (asignada via extension API, verificada por la comunidad)
	PendingChasterTask    string     `json:"pending_chaster_task,omitempty"`
	ChasterTaskSessionID  string     `json:"chaster_task_session_id,omitempty"`
	ChasterTaskLockID     string     `json:"chaster_task_lock_id,omitempty"`
	ChasterTaskAssignedAt *time.Time `json:"chaster_task_assigned_at,omitempty"`
	ChasterTaskDBID       string     `json:"chaster_task_db_id,omitempty"`
}

// GetObedienceLevel devuelve el nivel de obediencia (0-3) según el streak actual
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
