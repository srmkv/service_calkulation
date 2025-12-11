package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"saas-calc-backend/internal/domain"
)

// --- Конфиг калькулятора расстояний ---

type DistanceConfigDTO struct {
	BasePrice      float64            `json:"basePrice"`
	PricePerKm     float64            `json:"pricePerKm"`
	LoadingPrice   float64            `json:"loadingPrice"`
	UnloadingPrice float64            `json:"unloadingPrice"`
	VehicleCoefs   map[string]float64 `json:"vehicleCoefs"`
}

// GET/POST /api/distance/config
func (e *Env) HandleDistanceConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := e.DistanceConfig
		if cfg == nil {
			cfg = domain.NewDefaultDistanceConfig()
			e.DistanceConfig = cfg
		}
		e.writeJSON(w, cfg)

	case http.MethodPost:
		defer r.Body.Close()

		var req DistanceConfigDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}

		if e.DistanceConfig == nil {
			e.DistanceConfig = domain.NewDefaultDistanceConfig()
		}
		cfg := e.DistanceConfig

		cfg.BasePrice = req.BasePrice
		cfg.PricePerKm = req.PricePerKm
		cfg.LoadingPrice = req.LoadingPrice
		cfg.UnloadingPrice = req.UnloadingPrice

		if cfg.VehicleCoefs == nil {
			cfg.VehicleCoefs = map[string]float64{}
		}
		if req.VehicleCoefs != nil {
			for k, v := range req.VehicleCoefs {
				cfg.VehicleCoefs[k] = v
			}
		}

		e.writeJSON(w, cfg)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Расчёт маршрута через Nominatim + OSRM ---

type DistanceCalcRequest struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Vehicle      string `json:"vehicle"`
	RoundTrip    bool   `json:"roundTrip"`
	CalculatorID string `json:"calculatorId"`
}

type RoutePoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type DistanceCalcResponse struct {
	DistanceOneWayKm float64      `json:"distanceOneWayKm"`
	DistanceTotalKm  float64      `json:"distanceTotalKm"`
	PriceBase        float64      `json:"priceBase"`
	PriceKm          float64      `json:"priceKm"`
	PriceLoad        float64      `json:"priceLoad"`
	PriceTotal       float64      `json:"priceTotal"`
	Route            []RoutePoint `json:"route"` // маршрут для отрисовки на карте
}

// simple Nominatim response
type nominatimResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

func (e *Env) geocodeAddress(addr string) (lat, lon float64, err error) {
	base := e.NominatimBaseURL
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}

	u, err := url.Parse(base + "/search")
	if err != nil {
		return 0, 0, fmt.Errorf("bad nominatim base url: %w", err)
	}

	q := u.Query()
	q.Set("format", "json")
	q.Set("limit", "1")
	q.Set("q", addr)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, 0, fmt.Errorf("build nominatim request: %w", err)
	}
	req.Header.Set("User-Agent", "saas-calc/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("nominatim request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("nominatim status: %s", resp.Status)
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, fmt.Errorf("decode nominatim: %w", err)
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("nominatim: no results for %q", addr)
	}

	lat, err = strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lat: %w", err)
	}
	lon, err = strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lon: %w", err)
	}

	return lat, lon, nil
}

// OSRM ответ с геометрией
type osrmRouteResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"` // meters
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"` // [lon, lat]
		} `json:"geometry"`
	} `json:"routes"`
	Code string `json:"code"`
}

// osrmRoute возвращает дистанцию и набор точек маршрута (lat,lon)
func (e *Env) osrmRoute(lat1, lon1, lat2, lon2 float64) (float64, []RoutePoint, error) {
	base := e.OSRMBaseURL
	if base == "" {
		base = "https://router.project-osrm.org"
	}

	u, err := url.Parse(base + "/route/v1/driving/")
	if err != nil {
		return 0, nil, fmt.Errorf("bad osrm base url: %w", err)
	}

	// OSRM ожидает lon,lat
	u.Path += fmt.Sprintf("%f,%f;%f,%f", lon1, lat1, lon2, lat2)
	q := u.Query()
	q.Set("overview", "full")
	q.Set("geometries", "geojson")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return 0, nil, fmt.Errorf("osrm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("osrm status: %s", resp.Status)
	}

	var data osrmRouteResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, nil, fmt.Errorf("decode osrm: %w", err)
	}

	if data.Code != "" && data.Code != "Ok" {
		return 0, nil, fmt.Errorf("osrm code: %s", data.Code)
	}
	if len(data.Routes) == 0 {
		return 0, nil, fmt.Errorf("osrm: no routes")
	}

	route := make([]RoutePoint, 0, len(data.Routes[0].Geometry.Coordinates))
	for _, c := range data.Routes[0].Geometry.Coordinates {
		if len(c) < 2 {
			continue
		}
		// [lon, lat] -> {lat, lon}
		route = append(route, RoutePoint{
			Lat: c[1],
			Lon: c[0],
		})
	}

	return data.Routes[0].Distance, route, nil
}

// POST /api/distance/calc
func (e *Env) HandleDistanceCalc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req DistanceCalcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.From == "" || req.To == "" {
		http.Error(w, "from/to required", http.StatusBadRequest)
		return
	}

	cfg := e.DistanceConfig
	if cfg == nil {
		cfg = domain.NewDefaultDistanceConfig()
		e.DistanceConfig = cfg
	}

	lat1, lon1, err := e.geocodeAddress(req.From)
	if err != nil {
		http.Error(w, "geocode from: "+err.Error(), http.StatusBadRequest)
		return
	}
	lat2, lon2, err := e.geocodeAddress(req.To)
	if err != nil {
		http.Error(w, "geocode to: "+err.Error(), http.StatusBadRequest)
		return
	}

	distMeters, route, err := e.osrmRoute(lat1, lon1, lat2, lon2)
	if err != nil {
		http.Error(w, "osrm route: "+err.Error(), http.StatusBadRequest)
		return
	}
	distKm := distMeters / 1000.0

	base := cfg.BasePrice
	pricePerKm := cfg.PricePerKm
	loadSum := cfg.LoadingPrice + cfg.UnloadingPrice

	coef := 1.0
	if cfg.VehicleCoefs != nil && req.Vehicle != "" {
		if v, ok := cfg.VehicleCoefs[req.Vehicle]; ok && v > 0 {
			coef = v
		}
	}

	oneWayKm := distKm
	totalKm := distKm
	if req.RoundTrip {
		totalKm = distKm * 2
	}

	kmCost := totalKm * pricePerKm * coef
	total := base + kmCost + loadSum

	resp := DistanceCalcResponse{
		DistanceOneWayKm: oneWayKm,
		DistanceTotalKm:  totalKm,
		PriceBase:        base,
		PriceKm:          kmCost,
		PriceLoad:        loadSum,
		PriceTotal:       total,
		Route:            route,
	}

	// инкрементируем счётчик расчётов, если передан calculatorId
	if req.CalculatorID != "" {
		e.IncrementCalcCount(req.CalculatorID)

		// и отправляем уведомление в Telegram (если у владельца есть chat_id и токен)
		e.NotifyTelegramDistanceCalc(
			r.Context(),
			req.CalculatorID,
			req.From,
			req.To,
			req.Vehicle,
			req.RoundTrip,
			resp.DistanceTotalKm,
			resp.PriceTotal,
		)
	}

	e.writeJSON(w, resp)
}
