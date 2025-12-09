package app

import (
    "net/http"

    "saas-calc-backend/internal/domain"
    "saas-calc-backend/internal/handlers"
)

type App struct {
    mux *http.ServeMux
    Env *handlers.Env
}

func New() *App {
    mux := http.NewServeMux()

    plans := domain.DefaultPlans()
    users := domain.MockUsers(plans)
    calculators := domain.MockCalculators(users)

    env := &handlers.Env{
        LayeredConfig:  domain.NewDefaultLayeredConfig(),
        DistanceConfig: domain.NewDefaultDistanceConfig(),
        UploadDir:      "../frontend/uploads",
        Plans:          plans,
        Users:          users,
        Calculators:    calculators,
        NextCalcID:     len(calculators) + 1,
        OSRMBaseURL:      "https://router.project-osrm.org",
    NominatimBaseURL: "https://nominatim.openstreetmap.org",
    }

    registerRoutes(mux, env)

    return &App{
        mux: mux,
        Env: env,
    }
}

func (a *App) Router() *http.ServeMux {
    return a.mux
}
