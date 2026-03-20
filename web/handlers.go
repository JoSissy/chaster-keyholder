package web

import (
	"net/http"
	"strconv"
	"time"

	"chaster-keyholder/models"
	"chaster-keyholder/storage"
)

// ── Shared base ───────────────────────────────────────────────────────────

type pageBase struct {
	Nav          string
	TelegramLink string
}

func (s *Server) base(nav string) pageBase {
	return pageBase{Nav: nav, TelegramLink: s.telegramLink}
}

// ── Dashboard ─────────────────────────────────────────────────────────────

type dashData struct {
	pageBase
	IsLocked        bool
	DaysLocked      int
	Streak          int
	ObedienceName   string
	ObedienceLevel  int
	TasksCompleted  int
	TasksFailed     int
	CompletionRate  int // % tareas completadas
	WeeklyDebt      int
	TimeAdded       int
	TimeRemoved     int
	PendingCheckin  bool
	HasCurrentTask  bool
	CurrentTaskDesc string
	CurrentTaskDue  time.Time
	RecentTasks     []*storage.Task
	OrgasmTotal     int
	OrgasmGranted   int
	OrgasmDenied    int
	GrantRate       int
	// Lock timing
	HasEndDate      bool
	LockEndISO      string     // for JS countdown
	LockStartISO    string     // for JS progress bar
	LockStartDate   *time.Time // for display
	LockEndDate     *time.Time // for display
	ProgressPct     int        // % del lock completado
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	st := s.loadState()
	isLocked := st.CurrentLockID != "" || st.DaysLocked > 0

	obLevel := models.GetObedienceLevel(st.TasksStreak)
	taskTotal := st.TasksCompleted + st.TasksFailed
	completionRate := 0
	if taskTotal > 0 {
		completionRate = st.TasksCompleted * 100 / taskTotal
	}

	total, granted, denied, _ := s.db.GetOrgasmStats()
	grantRate := 0
	if total > 0 {
		grantRate = granted * 100 / total
	}

	// Lock timing
	hasEndDate := st.LockEndDate != nil
	lockEndISO := ""
	lockStartISO := ""
	progressPct := 0
	if st.LockEndDate != nil {
		lockEndISO = st.LockEndDate.UTC().Format(time.RFC3339)
	}
	if st.LockStartDate != nil {
		lockStartISO = st.LockStartDate.UTC().Format(time.RFC3339)
		if st.LockEndDate != nil {
			total_ := st.LockEndDate.Sub(*st.LockStartDate)
			elapsed := time.Since(*st.LockStartDate)
			if total_ > 0 {
				pct := int(elapsed * 100 / total_)
				if pct < 0 {
					pct = 0
				} else if pct > 100 {
					pct = 100
				}
				progressPct = pct
			}
		}
	}

	d := dashData{
		pageBase:        s.base("dashboard"),
		IsLocked:        isLocked,
		DaysLocked:      st.DaysLocked,
		Streak:          st.TasksStreak,
		ObedienceName:   models.ObedienceLevelString(obLevel),
		ObedienceLevel:  obLevel,
		TasksCompleted:  st.TasksCompleted,
		TasksFailed:     st.TasksFailed,
		CompletionRate:  completionRate,
		WeeklyDebt:      st.WeeklyDebt,
		TimeAdded:       st.TotalTimeAddedHours,
		TimeRemoved:     st.TotalTimeRemovedHours,
		PendingCheckin:  st.PendingCheckin,
		OrgasmTotal:     total,
		OrgasmGranted:   granted,
		OrgasmDenied:    denied,
		GrantRate:       grantRate,
		HasEndDate:      hasEndDate,
		LockEndISO:      lockEndISO,
		LockStartISO:    lockStartISO,
		LockStartDate:   st.LockStartDate,
		LockEndDate:     st.LockEndDate,
		ProgressPct:     progressPct,
	}
	if st.CurrentTask != nil {
		d.HasCurrentTask = true
		d.CurrentTaskDesc = st.CurrentTask.Description
		d.CurrentTaskDue = st.CurrentTask.DueAt
	}
	recent, _ := s.db.GetRecentTasks(6)
	d.RecentTasks = recent

	s.render(w, dashboardHTML, d)
}

// ── Calendar ──────────────────────────────────────────────────────────────

type calDay struct {
	Day        int
	Date       time.Time
	IsToday    bool
	IsLocked   bool
	TaskStatus string // "" | "completed" | "failed" | "pending"
}

type calData struct {
	pageBase
	Year     int
	Month    int
	MonthStr string
	PrevURL  string
	NextURL  string
	Weeks    [][]calDay
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("COT", -5*3600)
	}
	now := time.Now().In(loc)

	year, _ := strconv.Atoi(r.URL.Query().Get("y"))
	month, _ := strconv.Atoi(r.URL.Query().Get("m"))
	if year == 0 {
		year = now.Year()
	}
	if month == 0 {
		month = int(now.Month())
	}

	// Clamp month to 1-12
	if month < 1 {
		month = 12
		year--
	} else if month > 12 {
		month = 1
		year++
	}

	prevM, prevY := month-1, year
	if prevM < 1 {
		prevM = 12
		prevY--
	}
	nextM, nextY := month+1, year
	if nextM > 12 {
		nextM = 1
		nextY++
	}

	locks, _ := s.db.GetLocks()
	tasks, _ := s.db.GetAllTasks()

	weeks := buildCalendar(year, month, locks, tasks, loc)

	cd := calData{
		pageBase: s.base("calendar"),
		Year:     year,
		Month:    month,
		MonthStr: monthName(month) + " " + strconv.Itoa(year),
		PrevURL:  "/calendar?y=" + strconv.Itoa(prevY) + "&m=" + strconv.Itoa(prevM),
		NextURL:  "/calendar?y=" + strconv.Itoa(nextY) + "&m=" + strconv.Itoa(nextM),
		Weeks:    weeks,
	}
	s.render(w, calendarHTML, cd)
}

func monthName(m int) string {
	names := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
		"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
	if m < 1 || m > 12 {
		return ""
	}
	return names[m]
}

func buildCalendar(year, month int, locks []*storage.Lock, tasks []*storage.Task, loc *time.Location) [][]calDay {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	daysInMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, loc).Day()
	today := time.Now().In(loc)
	todayStr := today.Format("2006-01-02")

	// ISO week: Monday = 0
	offset := (int(firstDay.Weekday()) + 6) % 7

	// Build task map by date string
	taskMap := map[string]*storage.Task{}
	for _, t := range tasks {
		key := t.AssignedAt.In(loc).Format("2006-01-02")
		if _, exists := taskMap[key]; !exists {
			taskMap[key] = t
		}
	}

	// Build flat slice of cells (padding + days + trailing padding)
	totalCells := offset + daysInMonth
	if totalCells%7 != 0 {
		totalCells += 7 - totalCells%7
	}
	cells := make([]calDay, totalCells)

	for d := 1; d <= daysInMonth; d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, loc)
		dateStr := date.Format("2006-01-02")
		t := taskMap[dateStr]
		taskStatus := ""
		if t != nil {
			taskStatus = t.Status
		}
		cells[offset+d-1] = calDay{
			Day:        d,
			Date:       date,
			IsToday:    dateStr == todayStr,
			IsLocked:   dayIsLocked(date, locks),
			TaskStatus: taskStatus,
		}
	}

	// Split into weeks
	var weeks [][]calDay
	for i := 0; i < len(cells); i += 7 {
		weeks = append(weeks, cells[i:i+7])
	}
	return weeks
}

func dayIsLocked(day time.Time, locks []*storage.Lock) bool {
	dayEnd := day.Add(24 * time.Hour)
	for _, l := range locks {
		start := l.StartedAt
		if l.EndedAt == nil {
			// Still active
			if !day.Before(start) {
				return true
			}
		} else {
			if !day.Before(start) && day.Before(*l.EndedAt) && dayEnd.After(start) {
				return true
			}
		}
	}
	return false
}

// ── Tasks ─────────────────────────────────────────────────────────────────

type tasksData struct {
	pageBase
	Tasks     []*storage.Task
	Total     int
	Completed int
	Failed    int
	Pending   int
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.db.GetAllTasks()
	d := tasksData{
		pageBase: s.base("tasks"),
		Tasks:    tasks,
		Total:    len(tasks),
	}
	for _, t := range tasks {
		switch t.Status {
		case "completed":
			d.Completed++
		case "failed":
			d.Failed++
		default:
			d.Pending++
		}
	}
	s.render(w, tasksHTML, d)
}

// ── Orgasms ───────────────────────────────────────────────────────────────

type orgasmsData struct {
	pageBase
	Entries   []*storage.OrgasmEntry
	Total     int
	Granted   int
	Denied    int
	GrantPct  int
}

func (s *Server) handleOrgasms(w http.ResponseWriter, r *http.Request) {
	entries, _ := s.db.GetAllOrgasmEntries()
	total, granted, denied, _ := s.db.GetOrgasmStats()
	grantPct := 0
	if total > 0 {
		grantPct = granted * 100 / total
	}
	s.render(w, orgasmsHTML, orgasmsData{
		pageBase: s.base("orgasms"),
		Entries:  entries,
		Total:    total,
		Granted:  granted,
		Denied:   denied,
		GrantPct: grantPct,
	})
}

// ── Toys ──────────────────────────────────────────────────────────────────

type toysData struct {
	pageBase
	Toys []*storage.Toy
}

func (s *Server) handleToys(w http.ResponseWriter, r *http.Request) {
	toys, _ := s.db.GetToys()
	s.render(w, toysHTML, toysData{
		pageBase: s.base("toys"),
		Toys:     toys,
	})
}
