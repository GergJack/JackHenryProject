# Weather-Service API Service

A simple HTTP server that provides weather forecasts using the National Weather Service API.

## Requirements Given

 * Accepts latitude and longitude coordinates  
 * Returns short forecast for today  
 * Categorizes temperature as hot/moderate/cold per specifications:
- **Hot**: ≥89.5°F
- **Moderate**: 60.8-89.4°F
- **Cold**: ≤60.7°F  
  * Uses National Weather Service API as data source

## Quick Start

```bash
# Run the server
go run main.go

# Test with NYC coordinates
curl "http://localhost:8080/weather?lat=40.7128&lon=-74.0060"
```

## API Endpoints

### GET /weather
**Parameters:**
- `lat` - Latitude (-90 to 90)
- `lon` - Longitude (-180 to 180)

**Example Response:**
```json
{
  "location": {"lat": 40.7128, "lng": -74.0060},
  "forecast": "Partly Cloudy",
  "temp_f": 75,
  "temp_type": "moderate",
  "details": "Partly cloudy, with a high near 75...",
  "last_updated": "2025-06-08 17:56:00 EST"
}
```

### GET /health
Simple health check endpoint.

### GET /
API documentation and temperature range info.

## Test Examples

```bash
#convenience endpoint 
curl "http://localhost:8080/austin"

# Different temperature ranges
curl "http://localhost:8080/weather?lat=25.7617&lon=-80.1918"   # Miami (likely hot)
curl "http://localhost:8080/weather?lat=47.6062&lon=-122.3321"  # Seattle (moderate)
curl "http://localhost:8080/weather?lat=64.2008&lon=-149.4937"  # Fairbanks (cold)

# Error cases
curl "http://localhost:8080/weather?lat=invalid&lon=-74"        # Bad input
curl "http://localhost:8080/weather?lat=95&lon=-74"             # Out of range
```

## Implementation Notes

This was built as a 1-hour take-home assignment. **Shortcuts taken due to time constraints:**

- **No tests** - Would add unit tests for temperature categorization and integration tests for API endpoints
- **Basic error handling** - Production would use structured logging (logrus/zap) and more detailed error responses
- **No graceful shutdown** - Would add signal handling and graceful shutdown in production
- **Hardcoded values** - Timeout, port, User-Agent would be configurable via environment variables
- **No caching** - NWS API responses could be cached for a few minutes to reduce load
- **No rate limiting** - Would add rate limiting to prevent abuse
- **Simplified validation** - More robust coordinate validation and error messages needed

## Architecture

The service uses a two-step process with the NWS API:
1. `GET /points/{lat},{lon}` - Get grid information and forecast URL
2. `GET {forecast_url}` - Get actual forecast data

Temperature categorization happens client-side after fetching the forecast.

## Dependencies

- Go standard library only
- No external dependencies for simplicity

## Production Considerations

For a production deployment, I would add:
- Configuration management (viper/env vars)
- Structured logging
- Metrics and monitoring (Prometheus)
- Health checks with dependency status
- Docker containerization
- Kubernetes deployment manifests
- Circuit breaker for NWS API calls
- Request tracing and correlation IDs
- API documentation (OpenAPI/Swagger)
