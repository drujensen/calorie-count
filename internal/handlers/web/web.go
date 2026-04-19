package web

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/drujensen/calorie-count/internal/middleware"
	"github.com/drujensen/calorie-count/internal/models"
	"github.com/drujensen/calorie-count/internal/repositories"
	"github.com/drujensen/calorie-count/internal/services"
	"github.com/drujensen/calorie-count/internal/templates"
)

var tmplFuncs = template.FuncMap{
	// heightFt returns the whole feet portion of a total-inches value.
	"heightFt": func(totalIn float64) int { return int(totalIn) / 12 },
	// heightInRem returns the remaining inches (after subtracting whole feet).
	"heightInRem": func(totalIn float64) float64 { return math.Mod(totalIn, 12) },
	// bmiClass returns a CSS class suffix based on the BMI value.
	"bmiClass": func(bmi float64) string {
		switch {
		case bmi <= 0:
			return ""
		case bmi < 18.5:
			return "bmi-underweight"
		case bmi < 25:
			return "bmi-normal"
		case bmi < 30:
			return "bmi-overweight"
		default:
			return "bmi-obese"
		}
	},
	"absInt":   func(n int) int { if n < 0 { return -n }; return n },
	"absFloat": func(f float64) float64 { if f < 0 { return -f }; return f },
	"sub":      func(a, b int) int { return a - b },
}

// MealGroup groups log entries under a named meal category.
type MealGroup struct {
	ID      string // "breakfast" | "lunch" | "dinner" — used as HTML element ID suffix
	Name    string // display label
	Entries []models.LogEntry
}

// groupEntriesByMeal splits entries into Breakfast/Lunch/Dinner buckets using
// the local timezone. Before 10:00 = Breakfast, 10:00–14:59 = Lunch, 15:00+ = Dinner.
// Entries are assumed to already be sorted oldest-first.
func groupEntriesByMeal(entries []models.LogEntry, loc *time.Location) []MealGroup {
	groups := []MealGroup{
		{ID: "breakfast", Name: "Breakfast"},
		{ID: "lunch", Name: "Lunch"},
		{ID: "dinner", Name: "Dinner"},
	}
	for _, e := range entries {
		local := e.LoggedAt.In(loc)
		min := local.Hour()*60 + local.Minute()
		switch {
		case min < 10*60:
			groups[0].Entries = append(groups[0].Entries, e)
		case min < 15*60:
			groups[1].Entries = append(groups[1].Entries, e)
		default:
			groups[2].Entries = append(groups[2].Entries, e)
		}
	}
	return groups
}

// PageData holds common data passed to all page templates.
type PageData struct {
	User        *models.User
	Error       string
	Success     string
	ActiveTab   string
	Entries     []models.LogEntry
	MealGroups  []MealGroup
	Summary     models.MacroSummary
	AIAvailable bool
	CSRFToken   string
	// Date navigation for the log page
	ViewDate    time.Time
	ViewDateStr string // "2006-01-02" for HTML date inputs
	PrevDate    string // "2006-01-02"
	NextDate    string // "2006-01-02"
	IsToday     bool
	TodayStr    string // "2006-01-02" — used as max on date picker
}

// OverviewPageData holds data for the overview page.
type OverviewPageData struct {
	PageData
	Summary    models.PeriodSummary
	CaloriePct int
	GoalData   models.GoalData
	HasBMRData bool
	// Three-zone bar (all as % of TDEE, capped at 120)
	BarGoalPct int // where the goal marker sits (CalorieGoal / TDEE * 100)
	BarFillPct int // actual fill width
	BarColor   string // "green", "yellow", or "red"
}

// GoalPageData holds data for the goal page.
type GoalPageData struct {
	PageData
	GoalData      models.GoalData
	WeightPoints  template.JS // JSON-encoded []WeightPoint for the chart
	HasWeightData bool
}

// SettingsPageData holds data for the settings page.
type SettingsPageData struct {
	PageData
	Success bool
}

// WebHandler handles HTML page requests with pre-parsed templates.
type WebHandler struct {
	pages        map[string]*template.Template
	authSvc      services.AuthService
	logSvc       services.LogService
	aiSvc        services.AIService
	weightSvc    services.WeightService
	userRepo     repositories.UserRepository
	isProduction bool
}

var pageNames = []string{
	"overview.html",
	"log.html",
	"goal.html",
	"settings.html",
	"login.html",
	"register.html",
}

// NewWebHandler parses each page together with the base template and returns
// a WebHandler. Templates are parsed once at startup and reused per request.
func NewWebHandler(authSvc services.AuthService, logSvc services.LogService, aiSvc services.AIService, weightSvc services.WeightService, userRepo repositories.UserRepository, isProduction bool) (*WebHandler, error) {
	pages := make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		tmpl, err := template.New("").Funcs(tmplFuncs).ParseFS(templates.Pages, "pages/base.html", "pages/"+name)
		if err != nil {
			return nil, err
		}
		tmpl, err = tmpl.ParseFS(templates.Components, "components/*.html")
		if err != nil {
			return nil, err
		}
		pages[name] = tmpl
	}
	return &WebHandler{
		pages:        pages,
		authSvc:      authSvc,
		logSvc:       logSvc,
		aiSvc:        aiSvc,
		weightSvc:    weightSvc,
		userRepo:     userRepo,
		isProduction: isProduction,
	}, nil
}

// SetupRoutes registers all web routes on the given mux.
func SetupRoutes(mux *http.ServeMux, authSvc services.AuthService, logSvc services.LogService, aiSvc services.AIService, weightSvc services.WeightService, userRepo repositories.UserRepository, isProduction bool, rateLimiter *middleware.RateLimiter) {
	handler, err := NewWebHandler(authSvc, logSvc, aiSvc, weightSvc, userRepo, isProduction)
	if err != nil {
		panic("failed to parse templates: " + err.Error())
	}

	requireAuth := middleware.RequireAuth(authSvc)
	csrf := middleware.CSRFMiddleware
	rl := rateLimiter.Limit

	mux.Handle("GET /", requireAuth(csrf(http.HandlerFunc(handler.Overview))))
	mux.Handle("GET /log", requireAuth(csrf(http.HandlerFunc(handler.Log))))
	mux.Handle("GET /goal", requireAuth(csrf(http.HandlerFunc(handler.Goal))))
	mux.Handle("GET /settings", requireAuth(csrf(http.HandlerFunc(handler.Settings))))
	mux.Handle("POST /settings", requireAuth(csrf(http.HandlerFunc(handler.PostSettings))))

	mux.Handle("GET /login", rl(csrf(http.HandlerFunc(handler.GetLogin))))
	mux.Handle("POST /login", rl(csrf(http.HandlerFunc(handler.PostLogin))))
	mux.Handle("GET /register", rl(csrf(http.HandlerFunc(handler.GetRegister))))
	mux.Handle("POST /register", rl(csrf(http.HandlerFunc(handler.PostRegister))))

	mux.Handle("POST /logout", requireAuth(csrf(http.HandlerFunc(handler.PostLogout))))

	mux.HandleFunc("GET /health", handler.Health)
}

func (h *WebHandler) render(w http.ResponseWriter, page string, data any) {
	tmpl, ok := h.pages[page]
	if !ok {
		http.Error(w, "unknown page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("template render error", "page", page, "error", err)
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

// Log renders the main log page (Tab 1).
func (h *WebHandler) Log(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	csrfToken := middleware.GetCSRFToken(r)
	aiAvailable := h.aiSvc.IsAvailable(r.Context())

	// If no tz param yet, send a tiny bootstrap page that resolves the client's
	// local date + timezone and immediately redirects. This prevents the flash
	// caused by the server rendering UTC "today" before JS could fix it.
	if r.URL.Query().Get("tz") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head><meta charset=utf-8>` + //nolint:errcheck
			`<script>var d=new Date(),tz=d.getTimezoneOffset(),` +
			`dt=d.getFullYear()+'-'+String(d.getMonth()+1).padStart(2,'0')+'-'+String(d.getDate()).padStart(2,'0');` +
			`location.replace('/log?date='+dt+'&tz='+tz);</script></head><body></body></html>`))
		return
	}
	loc := clientLocation(r)
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	viewDate := today
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc); err == nil {
			viewDate = parsed
		}
	}
	// Clamp to today — don't allow future dates.
	if viewDate.After(today) {
		viewDate = today
	}
	isToday := viewDate.Equal(today)

	pd := PageData{
		ActiveTab:   "log",
		User:        &user,
		AIAvailable: aiAvailable,
		CSRFToken:   csrfToken,
		ViewDate:    viewDate,
		ViewDateStr: viewDate.Format("2006-01-02"),
		PrevDate:    viewDate.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:    viewDate.AddDate(0, 0, 1).Format("2006-01-02"),
		IsToday:     isToday,
		TodayStr:    today.Format("2006-01-02"),
	}

	entries, err := h.logSvc.GetEntriesForDate(r.Context(), user.ID, viewDate)
	if err != nil {
		pd.Error = "failed to load entries"
		h.render(w, "log.html", pd)
		return
	}
	pd.Entries = entries
	pd.MealGroups = groupEntriesByMeal(entries, viewDate.Location())

	summary, err := h.logSvc.GetSummaryForDate(r.Context(), user.ID, viewDate)
	if err != nil {
		pd.Error = "failed to load summary"
		h.render(w, "log.html", pd)
		return
	}
	pd.Summary = summary

	h.render(w, "log.html", pd)
}

// Overview renders the overview page (Tab 1) with period summary and burn rate.
func (h *WebHandler) Overview(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	csrfToken := middleware.GetCSRFToken(r)

	if r.URL.Query().Get("tz") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head><meta charset=utf-8>` + //nolint:errcheck
			`<script>var tz=new Date().getTimezoneOffset(),p=new URLSearchParams(location.search);` +
			`p.set('tz',tz);location.replace('/?'+p.toString());</script></head><body></body></html>`))
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily"
	}

	now := time.Now().In(clientLocation(r))
	ps, err := h.logSvc.GetSummary(r.Context(), user.ID, period, now)
	if err != nil {
		h.render(w, "overview.html", OverviewPageData{
			PageData: PageData{ActiveTab: "overview", User: &user, Error: "failed to load summary", CSRFToken: csrfToken},
		})
		return
	}

	caloriePct := 0
	if ps.CalorieGoal > 0 {
		caloriePct = ps.AvgDailyCalories * 100 / ps.CalorieGoal
		if caloriePct > 150 {
			caloriePct = 150
		}
	}

	goalData, err := h.weightSvc.GetGoalData(r.Context(), user.ID, user.CalorieGoal, now)
	if err != nil {
		slog.Error("getting goal data", "error", err)
	}

	// Three-zone bar: scale everything against TDEE.
	// If TDEE is unknown, fall back to the old single-zone bar.
	barGoalPct, barFillPct := 0, caloriePct
	barColor := "green"
	tdee := goalData.TDEE
	if tdee > 0 && ps.CalorieGoal > 0 {
		barGoalPct = ps.CalorieGoal * 100 / tdee
		barFillPct = ps.AvgDailyCalories * 100 / tdee
		if barFillPct > 120 {
			barFillPct = 120
		}
		switch {
		case ps.AvgDailyCalories > tdee:
			barColor = "red"
		case ps.AvgDailyCalories > ps.CalorieGoal:
			barColor = "yellow"
		}
	}

	h.render(w, "overview.html", OverviewPageData{
		PageData:   PageData{ActiveTab: "overview", User: &user, CSRFToken: csrfToken},
		Summary:    ps,
		CaloriePct: caloriePct,
		GoalData:   goalData,
		HasBMRData: goalData.BMR > 0,
		BarGoalPct: barGoalPct,
		BarFillPct: barFillPct,
		BarColor:   barColor,
	})
}

// Goal renders the goal page with weight history chart and projection.
func (h *WebHandler) Goal(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	csrfToken := middleware.GetCSRFToken(r)

	now := time.Now().In(clientLocation(r))
	goalData, err := h.weightSvc.GetGoalData(r.Context(), user.ID, user.CalorieGoal, now)
	if err != nil {
		slog.Error("getting goal data", "error", err)
	}

	wpJSON := encodeWeightPoints(goalData.WeightPoints)

	h.render(w, "goal.html", GoalPageData{
		PageData:      PageData{ActiveTab: "goal", User: &user, CSRFToken: csrfToken},
		GoalData:      goalData,
		WeightPoints:  template.JS(wpJSON),
		HasWeightData: len(goalData.WeightPoints) > 0,
	})
}

// encodeWeightPoints JSON-encodes weight points for inline JS.
func encodeWeightPoints(points []models.WeightPoint) string {
	type jsPoint struct {
		T float64 `json:"t"` // unix ms
		W float64 `json:"w"` // weight lbs
	}
	out := make([]jsPoint, len(points))
	for i, p := range points {
		out[i] = jsPoint{
			T: float64(p.Date.UnixMilli()),
			W: p.WeightLbs,
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// activityMultiplier returns the TDEE multiplier for the given activity level.
func activityMultiplier(level string) float64 {
	switch level {
	case "light":
		return 1.375
	case "moderate":
		return 1.55
	case "very":
		return 1.725
	case "extra":
		return 1.9
	default: // "sedentary" or empty
		return 1.2
	}
}

// computeCalorieGoal derives the daily calorie goal from TDEE and the chosen rate.
// Falls back to sensible defaults when profile data is incomplete.
func computeCalorieGoal(weightLbs, heightIn float64, age int, sex, rate, activityLevel string) int {
	deficit := 0
	switch rate {
	case "lose_1":
		deficit = 500
	case "lose_2":
		deficit = 1000
	}

	if heightIn <= 0 || weightLbs <= 0 || age <= 0 {
		// Profile incomplete — use conservative defaults.
		return 2000 - deficit
	}

	weightKg := weightLbs / 2.2046
	heightCm := heightIn * 2.54
	base := 10*weightKg + 6.25*heightCm - 5*float64(age)
	bmrRaw := base - 161 // female default
	if sex == "male" {
		bmrRaw = base + 5
	}
	tdee := int(math.Round(bmrRaw * activityMultiplier(activityLevel)))
	goal := tdee - deficit
	if goal < 1200 {
		goal = 1200
	}
	return goal
}

// Settings renders the settings page.
func (h *WebHandler) Settings(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	csrfToken := middleware.GetCSRFToken(r)
	saved := r.URL.Query().Get("saved") == "1"
	h.render(w, "settings.html", SettingsPageData{
		PageData: PageData{ActiveTab: "settings", User: &user, CSRFToken: csrfToken},
		Success:  saved,
	})
}

// PostSettings handles the settings form submission.
func (h *WebHandler) PostSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	user, _ := middleware.UserFromContext(r.Context())
	csrfToken := middleware.GetCSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.render(w, "settings.html", SettingsPageData{
			PageData: PageData{ActiveTab: "settings", User: &user, Error: "invalid form data", CSRFToken: csrfToken},
		})
		return
	}

	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	weightLossRate := r.FormValue("weight_loss_rate")
	switch weightLossRate {
	case "maintain", "lose_1", "lose_2":
		// valid
	default:
		weightLossRate = "maintain"
	}

	activityLevel := r.FormValue("activity_level")
	switch activityLevel {
	case "sedentary", "light", "moderate", "very", "extra":
		// valid
	default:
		activityLevel = "sedentary"
	}

	weightStr := r.FormValue("current_weight_lbs")
	ageStr := r.FormValue("age")
	heightFtStr := r.FormValue("height_ft")
	heightInStr := r.FormValue("height_in")
	sex := r.FormValue("sex")

	weight := 0.0
	if weightStr != "" {
		var err error
		weight, err = strconv.ParseFloat(weightStr, 64)
		if err != nil || weight < 0 {
			h.render(w, "settings.html", SettingsPageData{
				PageData: PageData{ActiveTab: "settings", User: &user, Error: "invalid weight value", CSRFToken: csrfToken},
			})
			return
		}
	}

	targetWeightStr := r.FormValue("target_weight_lbs")
	targetWeight := 0.0
	if targetWeightStr != "" {
		var err error
		targetWeight, err = strconv.ParseFloat(targetWeightStr, 64)
		if err != nil || targetWeight < 0 {
			h.render(w, "settings.html", SettingsPageData{
				PageData: PageData{ActiveTab: "settings", User: &user, Error: "invalid target weight value", CSRFToken: csrfToken},
			})
			return
		}
	}

	age := 0
	if ageStr != "" {
		var err error
		age, err = strconv.Atoi(ageStr)
		if err != nil || age < 0 || age > 120 {
			h.render(w, "settings.html", SettingsPageData{
				PageData: PageData{ActiveTab: "settings", User: &user, Error: "age must be between 0 and 120", CSRFToken: csrfToken},
			})
			return
		}
	}

	heightFt := 0
	if heightFtStr != "" {
		var err error
		heightFt, err = strconv.Atoi(heightFtStr)
		if err != nil || heightFt < 0 || heightFt > 9 {
			h.render(w, "settings.html", SettingsPageData{
				PageData: PageData{ActiveTab: "settings", User: &user, Error: "invalid height (feet)", CSRFToken: csrfToken},
			})
			return
		}
	}

	heightIn := 0.0
	if heightInStr != "" {
		var err error
		heightIn, err = strconv.ParseFloat(heightInStr, 64)
		if err != nil || heightIn < 0 || heightIn >= 12 {
			h.render(w, "settings.html", SettingsPageData{
				PageData: PageData{ActiveTab: "settings", User: &user, Error: "invalid height (inches must be 0-11)", CSRFToken: csrfToken},
			})
			return
		}
	}

	totalInches := float64(heightFt)*12 + heightIn
	goal := computeCalorieGoal(weight, totalInches, age, sex, weightLossRate, activityLevel)

	if err := h.userRepo.UpdateProfile(r.Context(), user.ID, goal, weight, targetWeight, age, totalInches, sex, weightLossRate, activityLevel); err != nil {
		h.render(w, "settings.html", SettingsPageData{
			PageData: PageData{ActiveTab: "settings", User: &user, Error: "failed to save settings", CSRFToken: csrfToken},
		})
		return
	}

	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

// GetLogin renders the login form.
func (h *WebHandler) GetLogin(w http.ResponseWriter, r *http.Request) {
	h.render(w, "login.html", PageData{CSRFToken: middleware.GetCSRFToken(r)})
}

// PostLogin processes the login form submission.
func (h *WebHandler) PostLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", PageData{Error: "invalid form data", CSRFToken: middleware.GetCSRFToken(r)})
		return
	}

	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	session, err := h.authSvc.Login(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			h.render(w, "login.html", PageData{Error: "invalid credentials", CSRFToken: middleware.GetCSRFToken(r)})
			return
		}
		h.render(w, "login.html", PageData{Error: "an error occurred, please try again", CSRFToken: middleware.GetCSRFToken(r)})
		return
	}

	http.SetCookie(w, sessionCookie(session.Token, session.ExpiresAt, h.isProduction))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GetRegister renders the registration form.
func (h *WebHandler) GetRegister(w http.ResponseWriter, r *http.Request) {
	h.render(w, "register.html", PageData{CSRFToken: middleware.GetCSRFToken(r)})
}

// PostRegister processes the registration form submission.
func (h *WebHandler) PostRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseForm(); err != nil {
		h.render(w, "register.html", PageData{Error: "invalid form data", CSRFToken: middleware.GetCSRFToken(r)})
		return
	}

	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if password != confirmPassword {
		h.render(w, "register.html", PageData{Error: "passwords do not match", CSRFToken: middleware.GetCSRFToken(r)})
		return
	}

	_, err := h.authSvc.Register(r.Context(), email, password)
	if err != nil {
		h.render(w, "register.html", PageData{Error: err.Error(), CSRFToken: middleware.GetCSRFToken(r)})
		return
	}

	session, err := h.authSvc.Login(r.Context(), email, password)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, sessionCookie(session.Token, session.ExpiresAt, h.isProduction))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// PostLogout clears the session cookie and logs the user out.
func (h *WebHandler) PostLogout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseForm(); err == nil {
		if !middleware.ValidateCSRF(r) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
	}

	cookie, err := r.Cookie("session")
	if err == nil {
		_ = h.authSvc.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Health returns a JSON health check response.
func (h *WebHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}

// clientLocation returns a *time.Location derived from the "tz" form or query param
// set by the browser. The value is getTimezoneOffset() — minutes *behind* UTC
// (positive = west of UTC, e.g. PST = 480). Falls back to UTC.
func clientLocation(r *http.Request) *time.Location {
	tzStr := r.FormValue("tz")
	if tzStr == "" {
		tzStr = r.URL.Query().Get("tz")
	}
	offsetMin, err := strconv.Atoi(tzStr)
	if err != nil {
		return time.UTC
	}
	return time.FixedZone("client", -offsetMin*60)
}

func sessionCookie(token string, expires time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
}
