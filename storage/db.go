package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("error en migración: %w", err)
	}

	log.Println("✅ Base de datos iniciada")
	return db, nil
}

// Migrations
func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
	CREATE TABLE IF NOT EXISTS toys (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT,
		photo_url   TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS locks (
		id                  TEXT PRIMARY KEY,
		chaster_id          TEXT NOT NULL,
		started_at          DATETIME NOT NULL,
		ended_at            DATETIME,
		duration_hours      INTEGER DEFAULT 0,
		tasks_completed     INTEGER DEFAULT 0,
		tasks_failed        INTEGER DEFAULT 0,
		time_added_hours    INTEGER DEFAULT 0,
		time_removed_hours  INTEGER DEFAULT 0,
		events_count        INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id            TEXT PRIMARY KEY,
		lock_id       TEXT REFERENCES locks(id),
		description   TEXT NOT NULL,
		photo_url     TEXT,
		assigned_at   DATETIME NOT NULL,
		due_at        DATETIME NOT NULL,
		completed_at  DATETIME,
		status        TEXT DEFAULT 'pending',
		penalty_hours INTEGER DEFAULT 0,
		reward_hours  INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS events (
		id              TEXT PRIMARY KEY,
		lock_id         TEXT REFERENCES locks(id),
		type            TEXT NOT NULL,
		duration_minutes INTEGER DEFAULT 0,
		triggered_at    DATETIME NOT NULL,
		resolved_at     DATETIME,
		negotiated      INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS negotiations (
		id              TEXT PRIMARY KEY,
		lock_id         TEXT REFERENCES locks(id),
		request         TEXT NOT NULL,
		decision        TEXT NOT NULL,
		time_delta_hours INTEGER DEFAULT 0,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS state (
		id                      INTEGER PRIMARY KEY DEFAULT 1,
		current_lock_id         TEXT,
		current_task_id         TEXT,
		active_event_type       TEXT,
		active_event_expires_at DATETIME,
		tasks_streak            INTEGER DEFAULT 0,
		total_time_added_hours  INTEGER DEFAULT 0,
		total_time_removed_hours INTEGER DEFAULT 0,
		days_locked             INTEGER DEFAULT 0,
		manual_duration_seconds INTEGER DEFAULT 0,
		awaiting_lock_photo     INTEGER DEFAULT 0
	);

	INSERT OR IGNORE INTO state (id) VALUES (1);
	`)
	return err
}

// ── Toys ──────────────────────────────────────────────────────────────────

type Toy struct {
	ID          string
	Name        string
	Description string
	PhotoURL    string
	CreatedAt   time.Time
}

func (db *DB) SaveToy(t *Toy) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO toys (id, name, description, photo_url, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Description, t.PhotoURL, t.CreatedAt,
	)
	return err
}

func (db *DB) GetToys() ([]*Toy, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, created_at FROM toys ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toys []*Toy
	for rows.Next() {
		t := &Toy{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.PhotoURL, &t.CreatedAt); err != nil {
			return nil, err
		}
		toys = append(toys, t)
	}
	return toys, nil
}

func (db *DB) DeleteToy(id string) error {
	_, err := db.conn.Exec(`DELETE FROM toys WHERE id = ?`, id)
	return err
}

// ── Locks ─────────────────────────────────────────────────────────────────

type Lock struct {
	ID               string
	ChasterID        string
	StartedAt        time.Time
	EndedAt          *time.Time
	DurationHours    int
	TasksCompleted   int
	TasksFailed      int
	TimeAddedHours   int
	TimeRemovedHours int
	EventsCount      int
}

func (db *DB) SaveLock(l *Lock) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO locks
		(id, chaster_id, started_at, ended_at, duration_hours, tasks_completed, tasks_failed, time_added_hours, time_removed_hours, events_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.ChasterID, l.StartedAt, l.EndedAt, l.DurationHours,
		l.TasksCompleted, l.TasksFailed, l.TimeAddedHours, l.TimeRemovedHours, l.EventsCount,
	)
	return err
}

func (db *DB) UpdateLockEnd(id string, endedAt time.Time, tasksCompleted, tasksFailed, timeAdded, timeRemoved, eventsCount int) error {
	_, err := db.conn.Exec(`
		UPDATE locks SET ended_at=?, tasks_completed=?, tasks_failed=?,
		time_added_hours=?, time_removed_hours=?, events_count=?
		WHERE id=?`,
		endedAt, tasksCompleted, tasksFailed, timeAdded, timeRemoved, eventsCount, id,
	)
	return err
}

func (db *DB) GetLocks() ([]*Lock, error) {
	rows, err := db.conn.Query(`SELECT id, chaster_id, started_at, ended_at, duration_hours, tasks_completed, tasks_failed, time_added_hours, time_removed_hours, events_count FROM locks ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []*Lock
	for rows.Next() {
		l := &Lock{}
		if err := rows.Scan(&l.ID, &l.ChasterID, &l.StartedAt, &l.EndedAt, &l.DurationHours, &l.TasksCompleted, &l.TasksFailed, &l.TimeAddedHours, &l.TimeRemovedHours, &l.EventsCount); err != nil {
			return nil, err
		}
		locks = append(locks, l)
	}
	return locks, nil
}

// ── Tasks ─────────────────────────────────────────────────────────────────

type Task struct {
	ID           string
	LockID       string
	Description  string
	PhotoURL     string
	AssignedAt   time.Time
	DueAt        time.Time
	CompletedAt  *time.Time
	Status       string // "pending" | "completed" | "failed"
	PenaltyHours int
	RewardHours  int
}

func (db *DB) SaveTask(t *Task) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO tasks
		(id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.LockID, t.Description, t.PhotoURL, t.AssignedAt, t.DueAt,
		t.CompletedAt, t.Status, t.PenaltyHours, t.RewardHours,
	)
	return err
}

func (db *DB) GetTasksByLock(lockID string) ([]*Task, error) {
	rows, err := db.conn.Query(`SELECT id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours FROM tasks WHERE lock_id=? ORDER BY assigned_at`, lockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(&t.ID, &t.LockID, &t.Description, &t.PhotoURL, &t.AssignedAt, &t.DueAt, &t.CompletedAt, &t.Status, &t.PenaltyHours, &t.RewardHours); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ── Events ────────────────────────────────────────────────────────────────

type Event struct {
	ID              string
	LockID          string
	Type            string
	DurationMinutes int
	TriggeredAt     time.Time
	ResolvedAt      *time.Time
	Negotiated      bool
}

func (db *DB) SaveEvent(e *Event) error {
	neg := 0
	if e.Negotiated {
		neg = 1
	}
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO events (id, lock_id, type, duration_minutes, triggered_at, resolved_at, negotiated)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.LockID, e.Type, e.DurationMinutes, e.TriggeredAt, e.ResolvedAt, neg,
	)
	return err
}

// ── Negotiations ──────────────────────────────────────────────────────────

type Negotiation struct {
	ID             string
	LockID         string
	Request        string
	Decision       string
	TimeDeltaHours int
	CreatedAt      time.Time
}

func (db *DB) SaveNegotiation(n *Negotiation) error {
	_, err := db.conn.Exec(`
		INSERT INTO negotiations (id, lock_id, request, decision, time_delta_hours, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		n.ID, n.LockID, n.Request, n.Decision, n.TimeDeltaHours, n.CreatedAt,
	)
	return err
}

// ── State ─────────────────────────────────────────────────────────────────

type State struct {
	CurrentLockID         string
	CurrentTaskID         string
	ActiveEventType       string
	ActiveEventExpiresAt  *time.Time
	TasksStreak           int
	TotalTimeAddedHours   int
	TotalTimeRemovedHours int
	DaysLocked            int
	ManualDurationSeconds int
	AwaitingLockPhoto     bool
}

func (db *DB) LoadState() (*State, error) {
	s := &State{}
	var awaitingInt int
	var activeEventType sql.NullString
	var activeEventExpires sql.NullTime
	var currentLockID sql.NullString
	var currentTaskID sql.NullString

	err := db.conn.QueryRow(`
		SELECT current_lock_id, current_task_id, active_event_type, active_event_expires_at,
		tasks_streak, total_time_added_hours, total_time_removed_hours,
		days_locked, manual_duration_seconds, awaiting_lock_photo
		FROM state WHERE id=1`,
	).Scan(
		&currentLockID, &currentTaskID, &activeEventType, &activeEventExpires,
		&s.TasksStreak, &s.TotalTimeAddedHours, &s.TotalTimeRemovedHours,
		&s.DaysLocked, &s.ManualDurationSeconds, &awaitingInt,
	)
	if err != nil {
		return s, err
	}

	s.CurrentLockID = currentLockID.String
	s.CurrentTaskID = currentTaskID.String
	s.ActiveEventType = activeEventType.String
	s.AwaitingLockPhoto = awaitingInt == 1
	if activeEventExpires.Valid {
		t := activeEventExpires.Time
		s.ActiveEventExpiresAt = &t
	}
	return s, nil
}

func (db *DB) SaveState(s *State) error {
	awaitingInt := 0
	if s.AwaitingLockPhoto {
		awaitingInt = 1
	}
	_, err := db.conn.Exec(`
		UPDATE state SET
			current_lock_id=?, current_task_id=?,
			active_event_type=?, active_event_expires_at=?,
			tasks_streak=?, total_time_added_hours=?, total_time_removed_hours=?,
			days_locked=?, manual_duration_seconds=?, awaiting_lock_photo=?
		WHERE id=1`,
		nullStr(s.CurrentLockID), nullStr(s.CurrentTaskID),
		nullStr(s.ActiveEventType), s.ActiveEventExpiresAt,
		s.TasksStreak, s.TotalTimeAddedHours, s.TotalTimeRemovedHours,
		s.DaysLocked, s.ManualDurationSeconds, awaitingInt,
	)
	return err
}

// ── Stats ─────────────────────────────────────────────────────────────────

type Stats struct {
	TotalLocks            int
	TotalTasksCompleted   int
	TotalTasksFailed      int
	TotalTimeAddedHours   int
	TotalTimeRemovedHours int
	TotalEvents           int
	BestStreak            int
	TotalToys             int
}

func (db *DB) GetStats() (*Stats, error) {
	s := &Stats{}

	db.conn.QueryRow(`SELECT COUNT(*) FROM locks WHERE ended_at IS NOT NULL`).Scan(&s.TotalLocks)
	db.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='completed'`).Scan(&s.TotalTasksCompleted)
	db.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='failed'`).Scan(&s.TotalTasksFailed)
	db.conn.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&s.TotalEvents)
	db.conn.QueryRow(`SELECT COUNT(*) FROM toys`).Scan(&s.TotalToys)
	db.conn.QueryRow(`SELECT COALESCE(SUM(time_added_hours),0) FROM locks`).Scan(&s.TotalTimeAddedHours)
	db.conn.QueryRow(`SELECT COALESCE(SUM(time_removed_hours),0) FROM locks`).Scan(&s.TotalTimeRemovedHours)

	return s, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
