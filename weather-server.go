package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Quick weather API for the take-home assignment
// Shortcuts taken due to time constraints:
// - No tests 
// - No graceful shutdown handling
// - Basic error handling **would use structured logging)
// - No rate limiting or caching
// - Hardcoded timeout values

type WeatherResponse struct {
	Location    Location `json:"location"`
	Forecast    string   `json:"forecast"`
	Temp        int      `json:"temp_f"`
	TempType    string   `json:"temp_type"`
	Details     string   `json:"details,omitempty"`
	LastUpdated string   `json:"last_updated"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// PointsResponse NWS API structs - only including fields we actually use
type PointsResponse struct {
	Properties struct {
		Forecast string `json:"forecast"`
		// GridID, GridX, GridY available but not needed for this use case
	} `json:"properties"`
}

type ForecastResponse struct {
	Properties struct {
		Periods []ForecastPeriod `json:"periods"`
	} `json:"properties"`
}

// ForecastPeriod Simplified - NWS has way more fields ,but we only need as listed below
type ForecastPeriod struct {
	Name             string `json:"name"`
	IsDaytime        bool   `json:"isDaytime"`
	Temperature      int    `json:"temperature"`
	TemperatureUnit  string `json:"temperatureUnit"`
	ShortForecast    string `json:"shortForecast"`
	DetailedForecast string `json:"detailedForecast"`
}

type WeatherService struct {
	httpClient *http.Client
}

func NewWeatherService() *WeatherService {
	// TODO: make timeout configurable via env var
	return &WeatherService{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetForecast hits NWS API - two-step process unfortunately
func (ws *WeatherService) GetForecast(lat, lon float64) (*WeatherResponse, error) {
	// Step 1: Get the forecast URL from coordinates
	pointsURL := fmt.Sprintf("https://api.weather.gov/points/%.4f,%.4f", lat, lon)

	req, err := http.NewRequest("GET", pointsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create points request: %w", err)
	}

	// NWS returns 403 without User-Agent
	req.Header.Set("User-Agent", "WeatherApp/1.0 (contact@example.com)")

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch points data: %w", err)
	}
	defer resp.Body.Close()
	//would create error handling here

	if resp.StatusCode != http.StatusOK {
		// TODO: better error handling for different status codes
		return nil, fmt.Errorf("NWS points API returned status %d", resp.StatusCode)
	}

	var pointsResp PointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&pointsResp); err != nil {
		return nil, fmt.Errorf("failed to decode points response: %w", err)
	}

	// Step 2: Get actual forecast
	forecastURL := pointsResp.Properties.Forecast
	if forecastURL == "" {
		return nil, fmt.Errorf("no forecast URL available for this location")
	}

	return ws.fetchForecast(forecastURL, lat, lon)
}

// Split this out to keep main function readable
func (ws *WeatherService) fetchForecast(forecastURL string, lat, lon float64) (*WeatherResponse, error) {
	req, err := http.NewRequest("GET", forecastURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create forecast request: %w", err)
	}

	req.Header.Set("User-Agent", "WeatherApp/1.0 (contact@example.com)")

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch forecast data: %w", err)
	}
	defer resp.Body.Close()
	//would create error handling here

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NWS forecast API returned status %d", resp.StatusCode)
	}

	var forecastResp ForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&forecastResp); err != nil {
		return nil, fmt.Errorf("failed to decode forecast response: %w", err)
	}

	// Find today's forecast - usually the first daytime period
	var todayPeriod *ForecastPeriod
	for i := range forecastResp.Properties.Periods {
		period := &forecastResp.Properties.Periods[i]
		if strings.Contains(strings.ToLower(period.Name), "today") ||
			(todayPeriod == nil && period.IsDaytime) {
			//would create error handling here
			todayPeriod = period
			break
		}
	}

	if todayPeriod == nil && len(forecastResp.Properties.Periods) > 0 {
		// Fallback to first period if no "today" found
		todayPeriod = &forecastResp.Properties.Periods[0]
	}

	if todayPeriod == nil {
		return nil, fmt.Errorf("no forecast periods available")
	}

	// NWS usually returns F but just in case
	temperature := todayPeriod.Temperature
	if todayPeriod.TemperatureUnit == "C" {
		temperature = int(float64(temperature)*9/5 + 32)
	}

	tempType := getTempType(float64(temperature))

	return &WeatherResponse{
		Location:    Location{Lat: lat, Lng: lon},
		Forecast:    todayPeriod.ShortForecast,
		Temp:        temperature,
		TempType:    tempType,
		Details:     todayPeriod.DetailedForecast,
		LastUpdated: time.Now().Format("2006-01-02 15:04:05 MST"),
	}, nil
}

// Simple temp bucketing based on our requirements
func getTempType(temp float64) string {
	switch {
	case temp >= 89.5:
		return "hot"
	case temp >= 60.8:
		return "moderate"
	default:
		return "cold"
	}
}

// WeatherHandler handles HTTP requests for weather forecasts
type WeatherHandler struct {
	weatherService *WeatherService
}

// NewWeatherHandler creates a new weather handler
func NewWeatherHandler() *WeatherHandler {
	return &WeatherHandler{
		weatherService: NewWeatherService(),
	}
}

// ServeHTTP handles the /weather endpoint
func (wh *WeatherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		wh.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET supported")
		return
	}

	// Get coords from query params
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if latStr == "" || lonStr == "" {
		wh.sendError(w, http.StatusBadRequest, "Missing coords", "Need both lat and lon params")
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		wh.sendError(w, http.StatusBadRequest, "Bad latitude", "Must be a number")
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		wh.sendError(w, http.StatusBadRequest, "Bad longitude", "Must be a number")
		return
	}

	// Basic validation
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		wh.sendError(w, http.StatusBadRequest, "Invalid coords", "Check your lat/lon values")
		return
	}

	forecast, err := wh.weatherService.GetForecast(lat, lon)
	if err != nil {
		log.Printf("Forecast error: %v", err)
		wh.sendError(w, http.StatusInternalServerError, "API error", "Could not get forecast")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(forecast)
	//would create error handling here
}

// sendError writes an error response
func (wh *WeatherHandler) sendError(w http.ResponseWriter, statusCode int, error, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: error, Message: message})
	//would create error handling here
}

// Basic health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
	//would create error handling here
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	//would create error handling here
}

func main() {
	// Quick setup - in production would use proper config management
	weatherHandler := NewWeatherHandler()

	http.Handle("/weather", weatherHandler)
	http.HandleFunc("/health", healthHandler)

	// Basic info endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Hardcoded for now - would move to config
		info := map[string]interface{}{
			"name":      "Weather API",
			"endpoints": []string{"/weather?lat=X&lon=Y", "/health"},
			"temp_ranges": map[string]string{
				"cold":     "≤60.7°F",
				"moderate": "60.8-89.4°F",
				"hot":      "≥89.5°F",
			},
		}
		json.NewEncoder(w).Encode(info)
		//would create error handling here
	})

	port := ":8080"
	log.Printf("Weather server listening on %s", port)
	// TODO: add graceful shutdown, signal handling
	log.Fatal(http.ListenAndServe(port, nil))
}
