package models

import "time"

// ChasterLock representa una sesión de castidad activa
type ChasterLock struct {
	ID            string     `json:"_id"`
	Status        string     `json:"status"`
	StartDate     time.Time  `json:"-"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	TotalDuration int64      `json:"totalDuration"`
	Title         string     `json:"title"`
	Combination   string     `json:"combination,omitempty"`
	Frozen        bool       `json:"isFrozen"`
}

// Task representa una tarea diaria asignada por el keyholder IA.
// PenaltyHours y RewardHours están en HORAS para consistencia.
type Task struct {
	ID            string    `json:"id"`
	Description   string    `json:"description"`
	AssignedAt    time.Time `json:"assigned_at"`
	DueAt         time.Time `json:"due_at"`
	Completed     bool      `json:"completed"`
	Failed        bool      `json:"failed"`
	PenaltyHours  int       `json:"penalty_hours"`
	RewardHours   int       `json:"reward_hours"`
	AwaitingPhoto bool      `json:"awaiting_photo"`
}

// GameResult representa el resultado de un minijuego
type GameResult struct {
	Dice1     int    `json:"dice1"`
	Dice2     int    `json:"dice2"`
	Total     int    `json:"total"`
	TimeDelta int    `json:"time_delta_minutes"`
	Message   string `json:"message"`
}

// Toy representa un juguete en el inventario
type Toy struct {
	Name    string    `json:"name"`
	AddedAt time.Time `json:"added_at"`
}

// IntensityLevel nivel de intensidad según días encerrada
type IntensityLevel int

const (
	IntensityLight    IntensityLevel = 1 // días 1-3
	IntensityModerate IntensityLevel = 2 // días 4-7
	IntensityIntense  IntensityLevel = 3 // días 8-14
	IntensityMaximum  IntensityLevel = 4 // días 15+
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

// GetIntensity calcula el nivel de intensidad según días encerrada
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

// AppState estado global de la app (persiste en state.json).
// TotalTimeAddedHours y TotalTimeRemovedHours están en HORAS.
type AppState struct {
	CurrentTask           *Task  `json:"current_task,omitempty"`
	TotalTimeAddedHours   int    `json:"total_time_added_hours"`
	TotalTimeRemovedHours int    `json:"total_time_removed_hours"`
	DaysLocked            int    `json:"days_locked"`
	Toys                  []Toy  `json:"toys"`
	AwaitingLockPhoto     bool   `json:"awaiting_lock_photo"`
	CurrentLockID         string `json:"current_lock_id"`
	ManualDurationSeconds int    `json:"manual_duration_seconds"`
	TasksCompleted        int    `json:"tasks_completed"`
	TasksFailed           int    `json:"tasks_failed"`
}

// LockSession datos de la sesión de lock activa creada por el bot
type LockSession struct {
	LockID    string    `json:"lock_id"`
	StartedAt time.Time `json:"started_at"`
	Duration  int       `json:"duration_hours"`
}