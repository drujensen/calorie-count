package handlers

import (
	"io/fs"
	"net/http"

	"github.com/drujensen/calorie-count/internal/handlers/api"
	"github.com/drujensen/calorie-count/internal/handlers/web"
	"github.com/drujensen/calorie-count/internal/middleware"
	"github.com/drujensen/calorie-count/internal/repositories"
	"github.com/drujensen/calorie-count/internal/services"
	appweb "github.com/drujensen/calorie-count/web"
)

// SetupRoutes registers all application routes on the given mux.
func SetupRoutes(mux *http.ServeMux, authSvc services.AuthService, logSvc services.LogService, aiSvc services.AIService, weightSvc services.WeightService, userRepo repositories.UserRepository, isProduction bool, rateLimiter *middleware.RateLimiter) {
	api.SetupRoutes(mux, authSvc, logSvc, aiSvc, weightSvc)
	web.SetupRoutes(mux, authSvc, logSvc, aiSvc, weightSvc, userRepo, isProduction, rateLimiter)

	sub, _ := fs.Sub(appweb.StaticFS, "public")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
}
