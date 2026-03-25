package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"chaster-keyholder/models"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	conn *sql.DB
}

func NewDB(url string) (*DB, error) {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("error conectando a la base de datos: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("error en migración: %w", err)
	}

	log.Println("✅ Base de datos iniciada")
	return db, nil
}

// migrate aplica todas las migraciones pendientes en orden.
// Cada migración se ejecuta exactamente una vez y se registra en schema_version.
func (db *DB) migrate() error {
	db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)`)

	var version int
	db.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)

	migrations := []struct {
		version int
		label   string
		fn      func() error
	}{
		{1, "schema base", db.migrateV1},
		{2, "fix orgasm_log + columnas faltantes", db.migrateV2},
		{3, "conversation_summaries — memoria emocional", db.migrateV3},
	}

	for _, m := range migrations {
		if version >= m.version {
			continue
		}
		log.Printf("[migrate] aplicando v%d: %s", m.version, m.label)
		if err := m.fn(); err != nil {
			return fmt.Errorf("migration v%d (%s): %w", m.version, m.label, err)
		}
		db.conn.Exec(`INSERT INTO schema_version (version) VALUES ($1)`, m.version)
		version = m.version
		log.Printf("[migrate] v%d aplicada ✓", m.version)
	}
	return nil
}

// migrateV1 crea todas las tablas base con el schema limpio y definitivo.
//
// Descripción de tablas:
//
//	toys             — juguetes registrados (jaulas, plugs, vibradores, etc.)
//	locks            — historial de sesiones de castidad (una fila por sesión)
//	tasks            — tareas diarias asignadas (foto_url = evidencia en Cloudinary)
//	chaster_tasks    — tareas comunitarias de Chaster (verificadas por la comunidad, no por IA)
//	clothing         — prendas del guardarropa de Jolie
//	outfit_log       — registro de outfits diarios asignados con foto aprobada
//	events           — eventos random ejecutados (freeze, hidetime, pillory)
//	negotiations     — historial de negociaciones de tiempo (/removetime)
//	permission_log   — historial de solicitudes de permiso de orgasmo con resultado
//	orgasm_log       — orgasmos reportados con /came (method, toy usado, si era permitido)
//	session_state    — copia de seguridad de contadores críticos (espejo de AppState)
//	                   solo tiene UNA fila con id='current'
//	contracts        — contratos generados al inicio de cada sesión (/contract)
//	contract_rules   — reglas individuales del contrato activo con su castigo asociado
//	checkins         — historial de check-ins espontáneos (verification_code, tiempo respuesta)
//	chat_history     — historial de conversación del chat libre con Papi (para contexto)
//	violations_log   — infracciones al contrato detectadas por la IA en el chat libre
//	schema_version   — tabla de control de migraciones (una fila por migración aplicada)
//	bot_state        — KV store para el AppState serializado (reemplaza state.json)
//
// Todos los timestamps se guardan en UTC. Las comparaciones de "hoy" se hacen
// en COT (cotLocation) en bot.go antes de persistir.
func (db *DB) migrateV1() error {
	_, err := db.conn.Exec(`
	CREATE TABLE IF NOT EXISTS toys (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		description     TEXT DEFAULT '',
		photo_url       TEXT DEFAULT '',
		photo_public_id TEXT DEFAULT '',
		type            TEXT DEFAULT 'other',
		in_use          INTEGER DEFAULT 0,
		created_at      TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS locks (
		id                  TEXT PRIMARY KEY,
		chaster_id          TEXT NOT NULL,
		started_at          TIMESTAMPTZ NOT NULL,
		ended_at            TIMESTAMPTZ,
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
		photo_url     TEXT DEFAULT '',
		assigned_at   TIMESTAMPTZ NOT NULL,
		due_at        TIMESTAMPTZ NOT NULL,
		completed_at  TIMESTAMPTZ,
		status        TEXT DEFAULT 'pending',
		penalty_hours INTEGER DEFAULT 0,
		reward_hours  INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS chaster_tasks (
		id          TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		photo_url   TEXT DEFAULT '',
		result      TEXT DEFAULT 'pending',
		assigned_at TIMESTAMPTZ NOT NULL,
		resolved_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS clothing (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		description     TEXT DEFAULT '',
		photo_url       TEXT DEFAULT '',
		photo_public_id TEXT DEFAULT '',
		type            TEXT DEFAULT 'other',
		added_at        TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS outfit_log (
		id          TEXT PRIMARY KEY,
		date        TEXT NOT NULL,
		outfit_desc TEXT NOT NULL,
		pose_desc   TEXT DEFAULT '',
		photo_url   TEXT DEFAULT '',
		comment     TEXT DEFAULT '',
		created_at  TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS events (
		id               TEXT PRIMARY KEY,
		lock_id          TEXT REFERENCES locks(id),
		type             TEXT NOT NULL,
		duration_minutes INTEGER DEFAULT 0,
		triggered_at     TIMESTAMPTZ NOT NULL,
		resolved_at      TIMESTAMPTZ,
		negotiated       INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS negotiations (
		id               TEXT PRIMARY KEY,
		lock_id          TEXT REFERENCES locks(id),
		request          TEXT NOT NULL,
		decision         TEXT NOT NULL,
		time_delta_hours INTEGER DEFAULT 0,
		created_at       TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS permission_log (
		id             TEXT PRIMARY KEY,
		granted        INTEGER NOT NULL DEFAULT 0,
		outcome        TEXT DEFAULT '',
		user_message   TEXT DEFAULT '',
		senor_response TEXT DEFAULT '',
		condition_text TEXT DEFAULT '',
		streak_at_time INTEGER DEFAULT 0,
		days_locked    INTEGER DEFAULT 0,
		created_at     TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS orgasm_log (
		id                 TEXT PRIMARY KEY,
		created_at         TIMESTAMPTZ DEFAULT NOW(),
		method             TEXT DEFAULT '',
		toy_id             TEXT DEFAULT '',
		toy_name           TEXT DEFAULT '',
		permitted          INTEGER DEFAULT 1,
		permission_outcome TEXT DEFAULT 'granted_cum',
		streak_at_time     INTEGER DEFAULT 0,
		days_locked        INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS session_state (
		id                       TEXT PRIMARY KEY DEFAULT 'current',
		tasks_streak             INTEGER DEFAULT 0,
		tasks_completed          INTEGER DEFAULT 0,
		tasks_failed             INTEGER DEFAULT 0,
		total_time_added_hours   INTEGER DEFAULT 0,
		total_time_removed_hours INTEGER DEFAULT 0,
		weekly_debt              INTEGER DEFAULT 0,
		weekly_debt_details      TEXT DEFAULT '[]',
		last_judgment_date       TEXT DEFAULT '',
		current_lock_id          TEXT DEFAULT '',
		updated_at               TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS contracts (
		id         TEXT PRIMARY KEY,
		lock_id    TEXT DEFAULT '',
		text       TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS checkins (
		id                 TEXT PRIMARY KEY,
		lock_id            TEXT DEFAULT '',
		requested_at       TIMESTAMPTZ NOT NULL,
		responded_at       TIMESTAMPTZ,
		photo_url          TEXT DEFAULT '',
		verification_code  TEXT DEFAULT '',
		status             TEXT DEFAULT 'pending',
		response_time_mins INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS chat_history (
		id         BIGSERIAL PRIMARY KEY,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS contract_rules (
		id         TEXT PRIMARY KEY,
		lock_id    TEXT DEFAULT '',
		rule_text  TEXT NOT NULL,
		punishment TEXT NOT NULL,
		hours      INTEGER DEFAULT 0,
		minutes    INTEGER DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS violations_log (
		id         TEXT PRIMARY KEY,
		rule_id    TEXT NOT NULL,
		rule_text  TEXT NOT NULL,
		punishment TEXT NOT NULL,
		hours      INTEGER DEFAULT 0,
		minutes    INTEGER DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_assigned_at       ON tasks(assigned_at);
	CREATE INDEX IF NOT EXISTS idx_permission_log_created  ON permission_log(created_at);
	CREATE INDEX IF NOT EXISTS idx_orgasm_log_created      ON orgasm_log(created_at);
	CREATE INDEX IF NOT EXISTS idx_checkins_requested_at   ON checkins(requested_at);
	CREATE INDEX IF NOT EXISTS idx_chat_history_created    ON chat_history(created_at);
	CREATE INDEX IF NOT EXISTS idx_violations_log_created  ON violations_log(created_at);

	CREATE TABLE IF NOT EXISTS bot_state (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	`)
	return err
}

// migrateV3 — tabla de resúmenes de conversación (memoria emocional de Papi).
func (db *DB) migrateV3() error {
	_, err := db.conn.Exec(`
	CREATE TABLE IF NOT EXISTS conversation_summaries (
		id            BIGSERIAL PRIMARY KEY,
		summary       TEXT NOT NULL,
		message_count INTEGER DEFAULT 0,
		created_at    TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_conv_summaries_created ON conversation_summaries(created_at);
	`)
	return err
}

// migrateV2 — versión simplificada para Postgres con IF NOT EXISTS.
func (db *DB) migrateV2() error {
	db.conn.Exec(`ALTER TABLE toys ADD COLUMN IF NOT EXISTS photo_public_id TEXT DEFAULT ''`)
	db.conn.Exec(`ALTER TABLE clothing ADD COLUMN IF NOT EXISTS photo_public_id TEXT DEFAULT ''`)
	db.conn.Exec(`ALTER TABLE permission_log ADD COLUMN IF NOT EXISTS outcome TEXT DEFAULT ''`)
	db.conn.Exec(`ALTER TABLE checkins ADD COLUMN IF NOT EXISTS verification_code TEXT DEFAULT ''`)
	return nil
}

// ── Toys ──────────────────────────────────────────────────────────────────

type Toy struct {
	ID            string
	Name          string
	Description   string
	PhotoURL      string
	PhotoPublicID string
	Type          string // "cage", "plug", "vibrator", "restraint", "other"
	InUse         bool
	CreatedAt     time.Time
}

func (db *DB) SaveToy(t *Toy) error {
	inUse := 0
	if t.InUse {
		inUse = 1
	}
	_, err := db.conn.Exec(`
		INSERT INTO toys (id, name, description, photo_url, photo_public_id, type, in_use, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			photo_url=EXCLUDED.photo_url,
			photo_public_id=EXCLUDED.photo_public_id,
			type=EXCLUDED.type,
			in_use=EXCLUDED.in_use,
			created_at=EXCLUDED.created_at`,
		t.ID, t.Name, t.Description, t.PhotoURL, t.PhotoPublicID, t.Type, inUse, t.CreatedAt,
	)
	return err
}

func (db *DB) GetToys() ([]*Toy, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, COALESCE(photo_public_id,''), type, in_use, created_at FROM toys ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toys []*Toy
	for rows.Next() {
		t := &Toy{}
		var inUseInt int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.PhotoURL, &t.PhotoPublicID, &t.Type, &inUseInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.InUse = inUseInt == 1
		toys = append(toys, t)
	}
	return toys, nil
}

func (db *DB) GetCages() ([]*Toy, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, photo_url, COALESCE(photo_public_id,''), type, in_use, created_at FROM toys WHERE type='cage' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toys []*Toy
	for rows.Next() {
		t := &Toy{}
		var inUseInt int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.PhotoURL, &t.PhotoPublicID, &t.Type, &inUseInt, &t.CreatedAt); err != nil {
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
	_, err := db.conn.Exec(`UPDATE toys SET in_use=$1 WHERE id=$2`, val, id)
	return err
}

func (db *DB) ClearAllInUse() error {
	_, err := db.conn.Exec(`UPDATE toys SET in_use=0`)
	return err
}

func (db *DB) DeleteToy(id string) error {
	_, err := db.conn.Exec(`DELETE FROM toys WHERE id = $1`, id)
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
		INSERT INTO locks
		(id, chaster_id, started_at, ended_at, duration_hours, tasks_completed, tasks_failed, time_added_hours, time_removed_hours, events_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			chaster_id=EXCLUDED.chaster_id,
			started_at=EXCLUDED.started_at,
			ended_at=EXCLUDED.ended_at,
			duration_hours=EXCLUDED.duration_hours,
			tasks_completed=EXCLUDED.tasks_completed,
			tasks_failed=EXCLUDED.tasks_failed,
			time_added_hours=EXCLUDED.time_added_hours,
			time_removed_hours=EXCLUDED.time_removed_hours,
			events_count=EXCLUDED.events_count`,
		l.ID, l.ChasterID, l.StartedAt, l.EndedAt, l.DurationHours,
		l.TasksCompleted, l.TasksFailed, l.TimeAddedHours, l.TimeRemovedHours, l.EventsCount,
	)
	return err
}

func (db *DB) UpdateLockEnd(id string, endedAt time.Time, tasksCompleted, tasksFailed, timeAdded, timeRemoved, eventsCount int) error {
	_, err := db.conn.Exec(`
		UPDATE locks SET ended_at=$1, tasks_completed=$2, tasks_failed=$3,
		time_added_hours=$4, time_removed_hours=$5, events_count=$6
		WHERE id=$7`,
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
		INSERT INTO tasks
		(id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			lock_id=EXCLUDED.lock_id,
			description=EXCLUDED.description,
			photo_url=EXCLUDED.photo_url,
			assigned_at=EXCLUDED.assigned_at,
			due_at=EXCLUDED.due_at,
			completed_at=EXCLUDED.completed_at,
			status=EXCLUDED.status,
			penalty_hours=EXCLUDED.penalty_hours,
			reward_hours=EXCLUDED.reward_hours`,
		t.ID, t.LockID, t.Description, t.PhotoURL, t.AssignedAt, t.DueAt,
		t.CompletedAt, t.Status, t.PenaltyHours, t.RewardHours,
	)
	return err
}

func (db *DB) GetTasksByLock(lockID string) ([]*Task, error) {
	rows, err := db.conn.Query(`SELECT id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours FROM tasks WHERE lock_id=$1 ORDER BY assigned_at`, lockID)
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
	rows, err := db.conn.Query(`SELECT id, lock_id, description, photo_url, assigned_at, due_at, completed_at, status, penalty_hours, reward_hours FROM tasks ORDER BY assigned_at DESC LIMIT $1`, n)
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

func (db *DB) GetAllPermissionEntries() ([]*PermissionEntry, error) {
	rows, err := db.conn.Query(`SELECT id, granted, COALESCE(outcome,''), user_message, senor_response, condition_text, streak_at_time, days_locked, created_at FROM permission_log ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*PermissionEntry
	for rows.Next() {
		e := &PermissionEntry{}
		var granted int
		if err := rows.Scan(&e.ID, &granted, &e.Outcome, &e.UserMessage, &e.SenorResponse, &e.Condition, &e.StreakAtTime, &e.DaysLocked, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Granted = granted == 1
		if e.Outcome == "" {
			if e.Granted {
				e.Outcome = "granted_cum"
			} else {
				e.Outcome = "denied"
			}
		}
		if e.Outcome == "granted" {
			e.Outcome = "granted_cum"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) GetRecentTaskDescriptions(n int) ([]string, error) {
	rows, err := db.conn.Query(`SELECT description FROM tasks ORDER BY assigned_at DESC LIMIT $1`, n)
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

// ── Permission Log ────────────────────────────────────────────────────────

type PermissionEntry struct {
	ID            string
	Outcome       string // "denied" | "granted_cum" | "granted_toys" | "punished"
	Granted       bool   // true only for granted_cum
	UserMessage   string
	SenorResponse string
	Condition     string
	StreakAtTime  int
	DaysLocked    int
	CreatedAt     time.Time
}

func (db *DB) SavePermissionEntry(e *PermissionEntry) error {
	id := fmt.Sprintf("perm-%d", time.Now().UnixNano())
	granted := 0
	if e.Outcome == "granted_cum" || e.Outcome == "granted" {
		granted = 1
	}
	outcome := e.Outcome
	if outcome == "" {
		if granted == 1 {
			outcome = "granted_cum"
		} else {
			outcome = "denied"
		}
	}
	_, err := db.conn.Exec(`
		INSERT INTO permission_log (id, granted, outcome, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, granted, outcome, e.UserMessage, e.SenorResponse, e.Condition, e.StreakAtTime, e.DaysLocked, time.Now(),
	)
	return err
}

func (db *DB) GetPermissionHistory(limit int) ([]*PermissionEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, granted, COALESCE(outcome,''), user_message, senor_response, condition_text, streak_at_time, days_locked, created_at
		FROM permission_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*PermissionEntry
	for rows.Next() {
		e := &PermissionEntry{}
		var granted int
		if err := rows.Scan(&e.ID, &granted, &e.Outcome, &e.UserMessage, &e.SenorResponse, &e.Condition, &e.StreakAtTime, &e.DaysLocked, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Granted = granted == 1
		if e.Outcome == "" {
			if e.Granted {
				e.Outcome = "granted_cum"
			} else {
				e.Outcome = "denied"
			}
		}
		if e.Outcome == "granted" {
			e.Outcome = "granted_cum"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetPermissionStats devuelve (total, granted_cum, granted_toys, denied).
// granted_cum incluye registros históricos con outcome='granted' para compatibilidad.
func (db *DB) GetPermissionStats() (total, grantedCum, grantedToys, denied int, err error) {
	err = db.conn.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN COALESCE(outcome,'') IN ('granted_cum','granted') OR (COALESCE(outcome,'') = '' AND granted=1) THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN COALESCE(outcome,'') = 'granted_toys' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN COALESCE(outcome,'') IN ('denied','punished','') AND granted=0 THEN 1 ELSE 0 END), 0)
		FROM permission_log`).Scan(&total, &grantedCum, &grantedToys, &denied)
	return
}

// ── Orgasm Log (real orgasms reported by Jolie) ────────────────────────────

// OrgasmEntry un orgasmo real o sesión de juguetes reportada por Jolie.
// Method="toy_session" indica sesión de juguetes sin orgasmo.
type OrgasmEntry struct {
	ID                string
	CreatedAt         time.Time
	Method            string // "nipples", "anal", "ruined", "manual", "other", "toy_session"
	ToyID             string
	ToyName           string
	Permitted         bool   // false = sin permiso de Papi (solo relevante para orgasmos reales)
	PermissionOutcome string // "granted_cum", "none", etc.
	StreakAtTime      int
	DaysLocked        int
}

// OrgasmStats estadísticas de orgasmos reales y sesiones de juguetes.
type OrgasmStats struct {
	Total         int            // orgasmos reales (excluye toy_session)
	WithToys      int
	WithoutToys   int
	Violations    int
	Methods       map[string]int // método → cantidad (solo orgasmos reales)
	LastMethod    string
	LastAt        *time.Time
	ToySessions   int    // sesiones de juguetes sin orgasmo
	FavToy        string // juguete más usado (orgasmos + sesiones)
	FavToyCount   int
}

func (db *DB) SaveOrgasmEntry(e *OrgasmEntry) error {
	id := fmt.Sprintf("orgasm-%d", time.Now().UnixNano())
	permitted := 1
	if !e.Permitted {
		permitted = 0
	}
	_, err := db.conn.Exec(`
		INSERT INTO orgasm_log (id, created_at, method, toy_id, toy_name, permitted, permission_outcome, streak_at_time, days_locked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, time.Now(), e.Method, e.ToyID, e.ToyName, permitted, e.PermissionOutcome, e.StreakAtTime, e.DaysLocked,
	)
	return err
}

func (db *DB) GetOrgasmHistory(limit int) ([]*OrgasmEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, created_at, method, COALESCE(toy_id,''), COALESCE(toy_name,''), permitted,
		       COALESCE(permission_outcome,'granted_cum'), streak_at_time, days_locked
		FROM orgasm_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*OrgasmEntry
	for rows.Next() {
		e := &OrgasmEntry{}
		var permitted int
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Method, &e.ToyID, &e.ToyName, &permitted,
			&e.PermissionOutcome, &e.StreakAtTime, &e.DaysLocked); err != nil {
			return nil, err
		}
		e.Permitted = permitted == 1
		events = append(events, e)
	}
	return events, nil
}

// GetDaysSinceLastOrgasm devuelve los días desde el último orgasmo real reportado.
// Excluye sesiones de juguetes (method='toy_session'). Devuelve -1 si nunca hubo uno.
func (db *DB) GetDaysSinceLastOrgasm() int {
	var createdAt time.Time
	err := db.conn.QueryRow(
		`SELECT created_at FROM orgasm_log WHERE method != 'toy_session' ORDER BY created_at DESC LIMIT 1`,
	).Scan(&createdAt)
	if err != nil {
		return -1
	}
	return int(time.Since(createdAt).Hours()) / 24
}

// GetOrgasmStats devuelve estadísticas completas de orgasmos reales y sesiones de juguetes.
func (db *DB) GetOrgasmStats() (*OrgasmStats, error) {
	stats := &OrgasmStats{Methods: make(map[string]int)}

	// Orgasmos reales (excluye toy_session)
	db.conn.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN toy_id != '' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN toy_id = '' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN permitted = 0 THEN 1 ELSE 0 END), 0)
		FROM orgasm_log WHERE method != 'toy_session'`,
	).Scan(&stats.Total, &stats.WithToys, &stats.WithoutToys, &stats.Violations)

	// Sesiones de juguetes
	db.conn.QueryRow(`SELECT COUNT(*) FROM orgasm_log WHERE method = 'toy_session'`).Scan(&stats.ToySessions)

	// Conteo por método (solo orgasmos reales)
	rows, err := db.conn.Query(`SELECT method, COUNT(*) FROM orgasm_log WHERE method != 'toy_session' GROUP BY method ORDER BY COUNT(*) DESC`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var method string
			var count int
			rows.Scan(&method, &count)
			stats.Methods[method] = count
		}
	}

	// Último orgasmo real
	var lastAt time.Time
	var lastMethod string
	if err2 := db.conn.QueryRow(`SELECT created_at, method FROM orgasm_log WHERE method != 'toy_session' ORDER BY created_at DESC LIMIT 1`).Scan(&lastAt, &lastMethod); err2 == nil {
		stats.LastAt = &lastAt
		stats.LastMethod = lastMethod
	}

	// Juguete favorito (orgasmos + sesiones con toy_name no vacío)
	var favToy string
	var favCount int
	if err3 := db.conn.QueryRow(`SELECT toy_name, COUNT(*) as c FROM orgasm_log WHERE toy_name != '' GROUP BY toy_name ORDER BY c DESC LIMIT 1`).Scan(&favToy, &favCount); err3 == nil {
		stats.FavToy = favToy
		stats.FavToyCount = favCount
	}

	return stats, nil
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
		VALUES ('current', $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (id) DO UPDATE SET
			tasks_streak=EXCLUDED.tasks_streak,
			tasks_completed=EXCLUDED.tasks_completed,
			tasks_failed=EXCLUDED.tasks_failed,
			total_time_added_hours=EXCLUDED.total_time_added_hours,
			total_time_removed_hours=EXCLUDED.total_time_removed_hours,
			weekly_debt=EXCLUDED.weekly_debt,
			weekly_debt_details=EXCLUDED.weekly_debt_details,
			last_judgment_date=EXCLUDED.last_judgment_date,
			current_lock_id=EXCLUDED.current_lock_id,
			updated_at=EXCLUDED.updated_at`,
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
		`INSERT INTO chaster_tasks (id, description, photo_url, result, assigned_at, resolved_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET
		 	description=EXCLUDED.description,
		 	photo_url=EXCLUDED.photo_url,
		 	result=EXCLUDED.result,
		 	assigned_at=EXCLUDED.assigned_at,
		 	resolved_at=EXCLUDED.resolved_at`,
		t.ID, t.Description, t.PhotoURL, t.Result, t.AssignedAt, t.ResolvedAt,
	)
	return err
}

func (db *DB) UpdateChasterTaskResult(id, result string, photoURL string, resolvedAt *time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE chaster_tasks SET result=$1, photo_url=CASE WHEN $2!='' THEN $2 ELSE photo_url END, resolved_at=$3 WHERE id=$4`,
		result, photoURL, resolvedAt, id,
	)
	return err
}

func (db *DB) GetRecentChasterTaskDescriptions(n int) ([]string, error) {
	rows, err := db.conn.Query(`SELECT description FROM chaster_tasks ORDER BY assigned_at DESC LIMIT $1`, n)
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
		 FROM chaster_tasks ORDER BY assigned_at DESC LIMIT $1`, n,
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
		`INSERT INTO clothing (id, name, description, photo_url, type, added_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET
		 	name=EXCLUDED.name,
		 	description=EXCLUDED.description,
		 	photo_url=EXCLUDED.photo_url,
		 	type=EXCLUDED.type,
		 	added_at=EXCLUDED.added_at`,
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
	_, err := db.conn.Exec(`DELETE FROM clothing WHERE id = $1`, id)
	return err
}

// ── Reset ──────────────────────────────────────────────────────────────────

// ResetAllTables borra todos los datos de todas las tablas.
func (db *DB) ResetAllTables() error {
	tables := []string{
		"toys", "locks", "tasks", "chaster_tasks", "clothing", "outfit_log",
		"events", "negotiations", "permission_log", "orgasm_log", "session_state",
		"checkins", "contracts", "bot_state",
	}
	for _, t := range tables {
		if _, err := db.conn.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("error limpiando %s: %w", t, err)
		}
	}
	return nil
}

// ── Gallery ────────────────────────────────────────────────────────────────

// GalleryPhoto represents a photo from any source for the gallery page.
type GalleryPhoto struct {
	URL      string
	Category string // "task" | "outfit" | "toy" | "clothing" | "chatask"
	Caption  string
	Date     time.Time
	Status   string // task status or chatask result
}

func (db *DB) GetGalleryPhotos() ([]*GalleryPhoto, error) {
	var photos []*GalleryPhoto

	// Tasks
	{
		rows, err := db.conn.Query(`SELECT COALESCE(description,''), COALESCE(photo_url,''), COALESCE(status,''), assigned_at FROM tasks WHERE photo_url != '' AND photo_url IS NOT NULL ORDER BY assigned_at DESC`)
		if err == nil {
			for rows.Next() {
				p := &GalleryPhoto{Category: "task"}
				rows.Scan(&p.Caption, &p.URL, &p.Status, &p.Date)
				photos = append(photos, p)
			}
			rows.Close()
		}
	}

	// Outfit log
	{
		rows, err := db.conn.Query(`SELECT COALESCE(outfit_desc,''), COALESCE(photo_url,''), created_at FROM outfit_log WHERE photo_url != '' AND photo_url IS NOT NULL ORDER BY created_at DESC`)
		if err == nil {
			for rows.Next() {
				p := &GalleryPhoto{Category: "outfit", Status: "confirmed"}
				rows.Scan(&p.Caption, &p.URL, &p.Date)
				photos = append(photos, p)
			}
			rows.Close()
		}
	}

	// Toys
	{
		rows, err := db.conn.Query(`SELECT COALESCE(name,''), COALESCE(photo_url,''), COALESCE(type,''), created_at FROM toys WHERE photo_url != '' AND photo_url IS NOT NULL ORDER BY created_at DESC`)
		if err == nil {
			for rows.Next() {
				p := &GalleryPhoto{Category: "toy"}
				rows.Scan(&p.Caption, &p.URL, &p.Status, &p.Date)
				photos = append(photos, p)
			}
			rows.Close()
		}
	}

	// Clothing
	{
		rows, err := db.conn.Query(`SELECT COALESCE(name,''), COALESCE(photo_url,''), COALESCE(type,''), added_at FROM clothing WHERE photo_url != '' AND photo_url IS NOT NULL ORDER BY added_at DESC`)
		if err == nil {
			for rows.Next() {
				p := &GalleryPhoto{Category: "clothing"}
				rows.Scan(&p.Caption, &p.URL, &p.Status, &p.Date)
				photos = append(photos, p)
			}
			rows.Close()
		}
	}

	// Chaster tasks
	{
		rows, err := db.conn.Query(`SELECT COALESCE(description,''), COALESCE(photo_url,''), COALESCE(result,''), assigned_at FROM chaster_tasks WHERE photo_url != '' AND photo_url IS NOT NULL ORDER BY assigned_at DESC`)
		if err == nil {
			for rows.Next() {
				p := &GalleryPhoto{Category: "chatask"}
				rows.Scan(&p.Caption, &p.URL, &p.Status, &p.Date)
				photos = append(photos, p)
			}
			rows.Close()
		}
	}

	// Sort all photos by date desc
	sort.Slice(photos, func(i, j int) bool {
		return photos[i].Date.After(photos[j].Date)
	})

	return photos, nil
}

// ── Contracts ─────────────────────────────────────────────────────────────

type Contract struct {
	ID        string
	LockID    string
	Text      string
	CreatedAt time.Time
}

func (db *DB) SaveContract(c *Contract) error {
	_, err := db.conn.Exec(
		`INSERT INTO contracts (id, lock_id, text, created_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET
		 	lock_id=EXCLUDED.lock_id,
		 	text=EXCLUDED.text,
		 	created_at=EXCLUDED.created_at`,
		c.ID, c.LockID, c.Text, c.CreatedAt,
	)
	return err
}

func (db *DB) GetLatestContract() (*Contract, error) {
	c := &Contract{}
	err := db.conn.QueryRow(
		`SELECT id, lock_id, text, created_at FROM contracts ORDER BY created_at DESC LIMIT 1`,
	).Scan(&c.ID, &c.LockID, &c.Text, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (db *DB) GetContractByLockID(lockID string) (*Contract, error) {
	c := &Contract{}
	err := db.conn.QueryRow(
		`SELECT id, lock_id, text, created_at FROM contracts WHERE lock_id=$1 ORDER BY created_at DESC LIMIT 1`,
		lockID,
	).Scan(&c.ID, &c.LockID, &c.Text, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// SeedPermissionGranted inserta un permiso concedido N días en el pasado.
func (db *DB) SeedPermissionGranted(daysAgo int) error {
	id := fmt.Sprintf("seed-perm-%d", time.Now().UnixNano())
	createdAt := time.Now().AddDate(0, 0, -daysAgo)
	_, err := db.conn.Exec(
		`INSERT INTO permission_log (id, granted, user_message, senor_response, condition_text, streak_at_time, days_locked, created_at)
		 VALUES ($1, 1, 'permiso', 'Concedido.', '', 0, 0, $2)`,
		id, createdAt,
	)
	return err
}

// ── Checkins ──────────────────────────────────────────────────────────────

type CheckinEntry struct {
	ID               string
	LockID           string
	RequestedAt      time.Time
	RespondedAt      *time.Time
	PhotoURL         string
	Status           string // "pending" | "submitted" | "missed"
	ResponseTimeMins int
	VerificationCode string
}

func (db *DB) SaveCheckin(c *CheckinEntry) error {
	_, err := db.conn.Exec(
		`INSERT INTO checkins (id, lock_id, requested_at, responded_at, photo_url, status, response_time_mins, verification_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO UPDATE SET
		 	lock_id=EXCLUDED.lock_id,
		 	requested_at=EXCLUDED.requested_at,
		 	responded_at=EXCLUDED.responded_at,
		 	photo_url=EXCLUDED.photo_url,
		 	status=EXCLUDED.status,
		 	response_time_mins=EXCLUDED.response_time_mins,
		 	verification_code=EXCLUDED.verification_code`,
		c.ID, c.LockID, c.RequestedAt, c.RespondedAt, c.PhotoURL, c.Status, c.ResponseTimeMins, c.VerificationCode,
	)
	return err
}

func (db *DB) UpdateCheckin(id, status, photoURL string, respondedAt *time.Time, responseTimeMins int) error {
	_, err := db.conn.Exec(
		`UPDATE checkins SET status=$1, photo_url=CASE WHEN $2!='' THEN $2 ELSE photo_url END, responded_at=$3, response_time_mins=$4 WHERE id=$5`,
		status, photoURL, respondedAt, responseTimeMins, id,
	)
	return err
}

func (db *DB) GetCheckinHistory(limit int) ([]*CheckinEntry, error) {
	rows, err := db.conn.Query(
		`SELECT id, lock_id, requested_at, responded_at, photo_url, status, response_time_mins
		 FROM checkins ORDER BY requested_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*CheckinEntry
	for rows.Next() {
		c := &CheckinEntry{}
		if err := rows.Scan(&c.ID, &c.LockID, &c.RequestedAt, &c.RespondedAt, &c.PhotoURL, &c.Status, &c.ResponseTimeMins); err != nil {
			return nil, err
		}
		entries = append(entries, c)
	}
	return entries, nil
}

func (db *DB) GetCheckinStats() (total, approved, missed int, avgResponseMins int, err error) {
	err = db.conn.QueryRow(`
		SELECT COUNT(*),
		       SUM(CASE WHEN status='submitted' OR status='approved' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status='missed' OR status='rejected' THEN 1 ELSE 0 END),
		       COALESCE(ROUND(AVG(CASE WHEN (status='submitted' OR status='approved') AND response_time_mins > 0 THEN response_time_mins END))::integer, 0)
		FROM checkins`).Scan(&total, &approved, &missed, &avgResponseMins)
	return
}

// GetDaysSinceLastPermission devuelve los días desde el último permiso concedido.
// Devuelve -1 si nunca hubo uno.
func (db *DB) GetDaysSinceLastPermission() int {
	var createdAt time.Time
	err := db.conn.QueryRow(
		`SELECT created_at FROM permission_log WHERE granted=1 ORDER BY created_at DESC LIMIT 1`,
	).Scan(&createdAt)
	if err != nil {
		return -1
	}
	return int(time.Since(createdAt).Hours()) / 24
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
		`INSERT INTO outfit_log (id, date, outfit_desc, pose_desc, photo_url, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
		 	date=EXCLUDED.date,
		 	outfit_desc=EXCLUDED.outfit_desc,
		 	pose_desc=EXCLUDED.pose_desc,
		 	photo_url=EXCLUDED.photo_url,
		 	comment=EXCLUDED.comment,
		 	created_at=EXCLUDED.created_at`,
		e.ID, e.Date, e.OutfitDesc, e.PoseDesc, e.PhotoURL, e.Comment, e.CreatedAt,
	)
	return err
}

func (db *DB) GetOutfitHistory(limit int) ([]*OutfitEntry, error) {
	rows, err := db.conn.Query(
		`SELECT id, date, outfit_desc, pose_desc, photo_url, comment, created_at
		 FROM outfit_log ORDER BY created_at DESC LIMIT $1`, limit,
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
		INSERT INTO events (id, lock_id, type, duration_minutes, triggered_at, resolved_at, negotiated)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			lock_id=EXCLUDED.lock_id,
			type=EXCLUDED.type,
			duration_minutes=EXCLUDED.duration_minutes,
			triggered_at=EXCLUDED.triggered_at,
			resolved_at=EXCLUDED.resolved_at,
			negotiated=EXCLUDED.negotiated`,
		e.ID, e.LockID, e.Type, e.DurationMinutes, e.TriggeredAt, e.ResolvedAt, neg,
	)
	return err
}

// GetEvents devuelve los últimos N eventos ordenados por fecha descendente.
func (db *DB) GetEvents(limit int) ([]*Event, error) {
	rows, err := db.conn.Query(`
		SELECT id, lock_id, type, duration_minutes, triggered_at, resolved_at, negotiated
		FROM events ORDER BY triggered_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*Event
	for rows.Next() {
		e := &Event{}
		var neg int
		if err := rows.Scan(&e.ID, &e.LockID, &e.Type, &e.DurationMinutes, &e.TriggeredAt, &e.ResolvedAt, &neg); err != nil {
			return nil, err
		}
		e.Negotiated = neg == 1
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventStats devuelve estadísticas globales de eventos.
func (db *DB) GetEventStats() (total, freeze, hidetime, pillory int) {
	db.conn.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&total)
	db.conn.QueryRow(`SELECT COUNT(*) FROM events WHERE type='freeze'`).Scan(&freeze)
	db.conn.QueryRow(`SELECT COUNT(*) FROM events WHERE type='hidetime'`).Scan(&hidetime)
	db.conn.QueryRow(`SELECT COUNT(*) FROM events WHERE type='pillory'`).Scan(&pillory)
	return
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
		VALUES ($1, $2, $3, $4, $5, $6)`,
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

// ── Chat History ───────────────────────────────────────────────────────────

func (db *DB) SaveChatMessage(role, content string) error {
	_, err := db.conn.Exec(`INSERT INTO chat_history (role, content, created_at) VALUES ($1, $2, $3)`, role, content, time.Now())
	return err
}

// GetRecentChatHistory devuelve los últimos n pares de mensajes.
// Si el último mensaje tiene más de maxIdleMinutes de antigüedad, limpia y devuelve nil (nueva conversación).
func (db *DB) GetRecentChatHistory(n int, maxIdleMinutes int) ([]models.ChatMessage, error) {
	var lastCreated time.Time
	err := db.conn.QueryRow(`SELECT created_at FROM chat_history ORDER BY id DESC LIMIT 1`).Scan(&lastCreated)
	if err != nil {
		return nil, nil // sin historial
	}
	if time.Since(lastCreated) > time.Duration(maxIdleMinutes)*time.Minute {
		db.conn.Exec(`DELETE FROM chat_history`)
		return nil, nil // expirado, nueva conversación
	}

	rows, err := db.conn.Query(`
		SELECT role, content FROM (
			SELECT id, role, content FROM chat_history ORDER BY id DESC LIMIT $1
		) AS recent ORDER BY id ASC`, n*2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (db *DB) ClearChatHistory() error {
	_, err := db.conn.Exec(`DELETE FROM chat_history`)
	return err
}

// GetChatHistoryCount devuelve cuántos mensajes hay en chat_history.
func (db *DB) GetChatHistoryCount() int {
	var count int
	db.conn.QueryRow(`SELECT COUNT(*) FROM chat_history`).Scan(&count)
	return count
}

// GetAllChatHistory devuelve todos los mensajes en chat_history en orden cronológico.
func (db *DB) GetAllChatHistory() ([]models.ChatMessage, error) {
	rows, err := db.conn.Query(`SELECT role, content FROM chat_history ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ── Conversation Summaries (memoria emocional) ──────────────────────────────

func (db *DB) SaveConversationSummary(summary string, messageCount int) error {
	_, err := db.conn.Exec(
		`INSERT INTO conversation_summaries (summary, message_count, created_at) VALUES ($1, $2, $3)`,
		summary, messageCount, time.Now(),
	)
	return err
}

// GetRecentConversationSummaries devuelve los últimos n resúmenes en orden cronológico.
func (db *DB) GetRecentConversationSummaries(n int) ([]models.ConversationSummary, error) {
	rows, err := db.conn.Query(`
		SELECT summary, message_count, created_at FROM (
			SELECT summary, message_count, created_at FROM conversation_summaries ORDER BY created_at DESC LIMIT $1
		) AS recent ORDER BY created_at ASC`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []models.ConversationSummary
	for rows.Next() {
		var s models.ConversationSummary
		if err := rows.Scan(&s.Summary, &s.MessageCount, &s.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// ── Contract Rules ─────────────────────────────────────────────────────────

func (db *DB) SaveContractRules(rules []models.ContractRule) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("SaveContractRules: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck — rollback es no-op si ya se hizo Commit

	if _, err := tx.Exec(`DELETE FROM contract_rules`); err != nil {
		return fmt.Errorf("SaveContractRules: delete: %w", err)
	}
	for _, r := range rules {
		if _, err := tx.Exec(
			`INSERT INTO contract_rules (id, lock_id, rule_text, punishment, hours, minutes, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			r.ID, r.LockID, r.RuleText, r.Punishment, r.Hours, r.Minutes, time.Now(),
		); err != nil {
			return fmt.Errorf("SaveContractRules: insert %s: %w", r.ID, err)
		}
	}
	return tx.Commit()
}

func (db *DB) GetActiveContractRules() ([]models.ContractRule, error) {
	rows, err := db.conn.Query(`SELECT id, lock_id, rule_text, punishment, hours, minutes FROM contract_rules ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.ContractRule
	for rows.Next() {
		var r models.ContractRule
		if err := rows.Scan(&r.ID, &r.LockID, &r.RuleText, &r.Punishment, &r.Hours, &r.Minutes); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) ClearContractRules() error {
	_, err := db.conn.Exec(`DELETE FROM contract_rules`)
	return err
}

// ── Violations Log ─────────────────────────────────────────────────────────

func (db *DB) LogViolation(ruleID, ruleText, punishment string, hours, minutes int) error {
	id := fmt.Sprintf("violation-%d", time.Now().UnixNano())
	_, err := db.conn.Exec(
		`INSERT INTO violations_log (id, rule_id, rule_text, punishment, hours, minutes, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, ruleID, ruleText, punishment, hours, minutes, time.Now(),
	)
	return err
}

// CountRecentViolations devuelve cuántas veces se violó una regla en las últimas N horas.
func (db *DB) CountRecentViolations(ruleID string, hoursBack int) int {
	var count int
	db.conn.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM violations_log WHERE rule_id=$1 AND created_at > NOW() - INTERVAL '%d hours'`, hoursBack),
		ruleID,
	).Scan(&count)
	return count
}

// ── App State (replaces state.json) ────────────────────────────────────────

// SaveAppState serializa el AppState completo como JSON en la tabla bot_state.
func (db *DB) SaveAppState(s *models.AppState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("error serializando AppState: %w", err)
	}
	_, err = db.conn.Exec(`
		INSERT INTO bot_state (key, value, updated_at)
		VALUES ('app_state', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`,
		string(data),
	)
	return err
}

// LoadAppState deserializa el AppState desde la tabla bot_state.
// Devuelve nil, nil si no hay estado guardado (primera ejecución).
func (db *DB) LoadAppState() (*models.AppState, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM bot_state WHERE key='app_state'`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s models.AppState
	if err := json.Unmarshal([]byte(value), &s); err != nil {
		return nil, fmt.Errorf("error deserializando AppState: %w", err)
	}
	return &s, nil
}
