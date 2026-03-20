package storage

import (
	"database/sql"
	"encoding/json"
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

	// Columnas añadidas en migraciones posteriores (ignora error si ya existen)
	db.conn.Exec(`ALTER TABLE chaster_tasks ADD COLUMN photo_url TEXT DEFAULT ''`)
	db.conn.Exec(`ALTER TABLE chaster_tasks ADD COLUMN result TEXT DEFAULT 'pending'`)
	db.conn.Exec(`ALTER TABLE chaster_tasks ADD COLUMN resolved_at DATETIME`)

	log.Println("✅ Base de datos iniciada")
	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
	CREATE TABLE IF NOT EXISTS toys (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT,
		photo_url   TEXT,
		type        TEXT DEFAULT 'other',
		in_use      INTEGER DEFAULT 0,
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

	CREATE TABLE IF NOT EXISTS chaster_tasks (
		id          TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		photo_url   TEXT DEFAULT '',
		result      TEXT DEFAULT 'pending',
		assigned_at DATETIME NOT NULL,
		resolved_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS clothing (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT,
		photo_url   TEXT,
		type        TEXT DEFAULT 'other',
		added_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS outfit_log (
		id           TEXT PRIMARY KEY,
		date         TEXT NOT NULL,
		outfit_desc  TEXT NOT NULL,
		pose_desc    TEXT DEFAULT '',
		photo_url    TEXT DEFAULT '',
		comment      TEXT DEFAULT '',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
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

	CREATE TABLE IF NOT EXISTS orgasm_log (
		id               TEXT PRIMARY KEY,
		granted          INTEGER NOT NULL DEFAULT 0,
		user_message     TEXT NOT NULL,
		senor_response   TEXT NOT NULL,
		condition_text   TEXT DEFAULT '',
		streak_at_time   INTEGER DEFAULT 0,
		days_locked      INTEGER DEFAULT 0,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS session_state (
		id                      TEXT PRIMARY KEY DEFAULT 'current',
		tasks_streak            INTEGER DEFAULT 0,
		tasks_completed         INTEGER DEFAULT 0,
		tasks_failed            INTEGER DEFAULT 0,
		total_time_added_hours  INTEGER DEFAULT 0,
		total_time_removed_hours INTEGER DEFAULT 0,
		weekly_debt             INTEGER DEFAULT 0,
		weekly_debt_details     TEXT DEFAULT '[]',
		last_judgment_date      TEXT DEFAULT '',
		current_lock_id         TEXT DEFAULT '',
		updated_at              DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	`)
	return err
}

// ── Toys ──────────────────────────────────────────────────────────────────

type Toy struct {
	ID          string
	Name        string
	Description string
	PhotoURL    string
	Type        string // "cage", "plug", "vibrator", "restraint", "other"
	InUse       bool
	CreatedAt   time.Time
}

func (db *DB) SaveToy(t *Toy) error {
	inUse := 0
	if t.InUse {
		inUse = 1
	}
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO toys (id, name, description, photo_url, type, in_use, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Description, t.PhotoURL, t.Type, inUse, t.CreatedAt,
	)
	return err
}

func (db *DB) GetToys() ([]*Toy, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, type, in_use, created_at FROM toys ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toys []*Toy
	for rows.Next() {
		t := &Toy{}
		var inUseInt int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.PhotoURL, &t.Type, &inUseInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.InUse = inUseInt == 1
		toys = append(toys, t)
	}
	return toys, nil
}

func (db *DB) GetCages() ([]*Toy, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, type, in_use, created_at FROM toys WHERE type='cage' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toys []*Toy
	for rows.Next() {
		t := &Toy{}
		var inUseInt int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.PhotoURL, &t.Type, &inUseInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.InUse = inUseInt == 1
		toys = append(toys, t)
	}
	return toys, nil
}

func (db *DB) SetToyInUse(id string, inUse bool) error {
	val := 0
	if inUse {
		val = 1
	}
	// Primero desmarcar todas las jaulas si vamos a marcar una
	if inUse {
		db.conn.Exec(`UPDATE toys SET in_use=0 WHERE type='cage'`)
	}
	_, err := db.conn.Exec(`UPDATE toys SET in_use=? WHERE id=?`, val, id)
	return err
}

func (db *DB) ClearAllInUse() error {
	_, err := db.conn.Exec(`UPDATE toys SET in_use=0`)
	return err
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

func (db *DB) GetRecentTasks(n int) ([]*Task, error) {
	rows, err := db.conn.Query(`SELECT id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours FROM tasks ORDER BY assigned_at DESC LIMIT ?`, n)
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

func (db *DB) GetAllTasks() ([]*Task, error) {
	rows, err := db.conn.Query(`SELECT id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours FROM tasks ORDER BY assigned_at DESC`)
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

func (db *DB) GetAllOrgasmEntries() ([]*OrgasmEntry, error) {
	rows, err := db.conn.Query(`SELECT id, granted, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at FROM orgasm_log ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*OrgasmEntry
	for rows.Next() {
		e := &OrgasmEntry{}
		var granted int
		if err := rows.Scan(&e.ID, &granted, &e.UserMessage, &e.SenorResponse, &e.Condition, &e.StreakAtTime, &e.DaysLocked, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Granted = granted == 1
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) GetRecentTaskDescriptions(n int) ([]string, error) {
	rows, err := db.conn.Query(`SELECT description FROM tasks ORDER BY assigned_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var descs []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		descs = append(descs, d)
	}
	return descs, nil
}

// ── Orgasm Log ────────────────────────────────────────────────────────────

type OrgasmEntry struct {
	ID            string
	Granted       bool
	UserMessage   string
	SenorResponse string
	Condition     string
	StreakAtTime  int
	DaysLocked    int
	CreatedAt     time.Time
}

func (db *DB) SaveOrgasmEntry(e *OrgasmEntry) error {
	id := fmt.Sprintf("orgasm-%d", time.Now().UnixNano())
	granted := 0
	if e.Granted {
		granted = 1
	}
	_, err := db.conn.Exec(`
		INSERT INTO orgasm_log (id, granted, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, granted, e.UserMessage, e.SenorResponse, e.Condition, e.StreakAtTime, e.DaysLocked, time.Now(),
	)
	return err
}

func (db *DB) GetOrgasmHistory(limit int) ([]*OrgasmEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, granted, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at
		FROM orgasm_log ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*OrgasmEntry
	for rows.Next() {
		e := &OrgasmEntry{}
		var granted int
		if err := rows.Scan(&e.ID, &granted, &e.UserMessage, &e.SenorResponse, &e.Condition, &e.StreakAtTime, &e.DaysLocked, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Granted = granted == 1
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) GetOrgasmStats() (total, granted, denied int, err error) {
	err = db.conn.QueryRow(`SELECT COUNT(*), SUM(granted), SUM(1-granted) FROM orgasm_log`).Scan(&total, &granted, &denied)
	return
}

// ── Session State ─────────────────────────────────────────────────────────

type SessionState struct {
	TasksStreak            int
	TasksCompleted         int
	TasksFailed            int
	TotalTimeAddedHours    int
	TotalTimeRemovedHours  int
	WeeklyDebt             int
	WeeklyDebtDetails      []string
	LastJudgmentDate       string
	CurrentLockID          string
}

func (db *DB) SaveSessionState(s *SessionState) error {
	details, _ := json.Marshal(s.WeeklyDebtDetails)
	_, err := db.conn.Exec(`
		INSERT INTO session_state
			(id, tasks_streak, tasks_completed, tasks_failed,
			 total_time_added_hours, total_time_removed_hours,
			 weekly_debt, weekly_debt_details, last_judgment_date, current_lock_id, updated_at)
		VALUES ('current', ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			tasks_streak=excluded.tasks_streak,
			tasks_completed=excluded.tasks_completed,
			tasks_failed=excluded.tasks_failed,
			total_time_added_hours=excluded.total_time_added_hours,
			total_time_removed_hours=excluded.total_time_removed_hours,
			weekly_debt=excluded.weekly_debt,
			weekly_debt_details=excluded.weekly_debt_details,
			last_judgment_date=excluded.last_judgment_date,
			current_lock_id=excluded.current_lock_id,
			updated_at=excluded.updated_at`,
		s.TasksStreak, s.TasksCompleted, s.TasksFailed,
		s.TotalTimeAddedHours, s.TotalTimeRemovedHours,
		s.WeeklyDebt, string(details), s.LastJudgmentDate, s.CurrentLockID,
	)
	return err
}

func (db *DB) LoadSessionState() (*SessionState, error) {
	row := db.conn.QueryRow(`SELECT
		tasks_streak, tasks_completed, tasks_failed,
		total_time_added_hours, total_time_removed_hours,
		weekly_debt, weekly_debt_details, last_judgment_date, current_lock_id
		FROM session_state WHERE id='current'`)

	var s SessionState
	var detailsJSON string
	err := row.Scan(
		&s.TasksStreak, &s.TasksCompleted, &s.TasksFailed,
		&s.TotalTimeAddedHours, &s.TotalTimeRemovedHours,
		&s.WeeklyDebt, &detailsJSON, &s.LastJudgmentDate, &s.CurrentLockID,
	)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(detailsJSON), &s.WeeklyDebtDetails)
	return &s, nil
}

// ── Chaster Tasks ─────────────────────────────────────────────────────────

type ChasterTask struct {
	ID          string
	Description string
	PhotoURL    string
	Result      string // "pending"|"verified"|"rejected"|"abandoned"|"timeout"
	AssignedAt  time.Time
	ResolvedAt  *time.Time
}

func (db *DB) SaveChasterTask(t *ChasterTask) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO chaster_tasks (id, description, photo_url, result, assigned_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Description, t.PhotoURL, t.Result, t.AssignedAt, t.ResolvedAt,
	)
	return err
}

func (db *DB) UpdateChasterTaskResult(id, result string, photoURL string, resolvedAt *time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE chaster_tasks SET result=?, photo_url=CASE WHEN ?!='' THEN ? ELSE photo_url END, resolved_at=? WHERE id=?`,
		result, photoURL, photoURL, resolvedAt, id,
	)
	return err
}

func (db *DB) GetRecentChasterTaskDescriptions(n int) ([]string, error) {
	rows, err := db.conn.Query(`SELECT description FROM chaster_tasks ORDER BY assigned_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var descs []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		descs = append(descs, d)
	}
	return descs, nil
}

func (db *DB) GetChasterTaskHistory(n int) ([]*ChasterTask, error) {
	rows, err := db.conn.Query(
		`SELECT id, description, photo_url, result, assigned_at, resolved_at
		 FROM chaster_tasks ORDER BY assigned_at DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*ChasterTask
	for rows.Next() {
		t := &ChasterTask{}
		if err := rows.Scan(&t.ID, &t.Description, &t.PhotoURL, &t.Result, &t.AssignedAt, &t.ResolvedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ── Clothing ──────────────────────────────────────────────────────────────

type ClothingItem struct {
	ID          string
	Name        string
	Description string
	PhotoURL    string
	Type        string // "lingerie"|"dress"|"top"|"bottom"|"shoes"|"accessory"|"other"
	AddedAt     time.Time
}

func (db *DB) SaveClothingItem(c *ClothingItem) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO clothing (id, name, description, photo_url, type, added_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Description, c.PhotoURL, c.Type, c.AddedAt,
	)
	return err
}

func (db *DB) GetClothingItems() ([]*ClothingItem, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, type, added_at FROM clothing ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*ClothingItem
	for rows.Next() {
		c := &ClothingItem{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.PhotoURL, &c.Type, &c.AddedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}

func (db *DB) DeleteClothingItem(id string) error {
	_, err := db.conn.Exec(`DELETE FROM clothing WHERE id = ?`, id)
	return err
}

// ── Reset ──────────────────────────────────────────────────────────────────

// ResetAllTables borra todos los datos de todas las tablas.
func (db *DB) ResetAllTables() error {
	tables := []string{
		"toys", "locks", "tasks", "chaster_tasks", "clothing", "outfit_log",
		"events", "negotiations", "orgasm_log", "session_state",
	}
	for _, t := range tables {
		if _, err := db.conn.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("error limpiando %s: %w", t, err)
		}
	}
	return nil
}

// SeedOrgasmDenial inserta una denegación de orgasmo N días en el pasado.
func (db *DB) SeedOrgasmDenial(daysAgo int) error {
	id := fmt.Sprintf("seed-orgasm-%d", time.Now().UnixNano())
	createdAt := time.Now().AddDate(0, 0, -daysAgo)
	_, err := db.conn.Exec(
		`INSERT INTO orgasm_log (id, granted, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at)
		 VALUES (?, 0, 'permiso', 'Denegado.', '', 0, 0, ?)`,
		id, createdAt,
	)
	return err
}

// ── Outfit log ─────────────────────────────────────────────────────────────

type OutfitEntry struct {
	ID         string
	Date       string // "2006-01-02" COT
	OutfitDesc string
	PoseDesc   string
	PhotoURL   string
	Comment    string
	CreatedAt  time.Time
}

func (db *DB) SaveOutfitEntry(e *OutfitEntry) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO outfit_log (id, date, outfit_desc, pose_desc, photo_url, comment, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Date, e.OutfitDesc, e.PoseDesc, e.PhotoURL, e.Comment, e.CreatedAt,
	)
	return err
}

func (db *DB) GetOutfitHistory(limit int) ([]*OutfitEntry, error) {
	rows, err := db.conn.Query(
		`SELECT id, date, outfit_desc, pose_desc, photo_url, comment, created_at
		 FROM outfit_log ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*OutfitEntry
	for rows.Next() {
		e := &OutfitEntry{}
		if err := rows.Scan(&e.ID, &e.Date, &e.OutfitDesc, &e.PoseDesc, &e.PhotoURL, &e.Comment, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
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

