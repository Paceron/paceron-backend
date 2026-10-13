// Package routematch contiene el route matching server-side de competencias
// route (gamificacion-core D10, Task 6): parseo del GeoJSON LineString,
// sampleo a pasos fijos y match % por Haversine. Geometría pura, sin I/O.
package routematch

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const earthRadiusMeters = 6371000.0

// SampleStepMeters es el paso fijo del sampleo de la ruta (D10: ~30-50 m por
// segmento).
const SampleStepMeters = 40.0

// Point es un punto geográfico en grados (lat/lng).
type Point struct {
	Lat float64
	Lng float64
}

// HaversineMeters distancia entre dos coordenadas en metros (Task 4: misma
// fórmula que peak_speed de feedback; vive acá como única fuente).
func HaversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusMeters * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ParseLineString parsea un GeoJSON LineString ("coordinates": [[lng, lat],
// ...]). Acepta un único vértice (ruta degenerada = un solo sample); rechaza
// type distinto, arrays vacíos o pares incompletos.
func ParseLineString(raw string) ([]Point, error) {
	if raw == "" {
		return nil, errors.New("route_geojson vacío")
	}
	var doc struct {
		Type        string      `json:"type"`
		Coordinates [][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("route_geojson no es JSON válido: %w", err)
	}
	if doc.Type != "LineString" {
		return nil, fmt.Errorf("route_geojson debe ser LineString, es %q", doc.Type)
	}
	if len(doc.Coordinates) == 0 {
		return nil, errors.New("route_geojson LineString sin coordenadas")
	}
	points := make([]Point, 0, len(doc.Coordinates))
	for i, c := range doc.Coordinates {
		if len(c) < 2 {
			return nil, fmt.Errorf("coordenada %d incompleta en route_geojson", i)
		}
		points = append(points, Point{Lat: c[1], Lng: c[0]})
	}
	return points, nil
}

// SampleRoute sub-divide cada segmento: emite cada vértice más los puntos
// intermedios a intervalos fijos de SampleStepMeters medidos desde el vértice
// inicial del segmento. El primer vértice ya sale en la primer iteración.
func SampleRoute(route []Point) []Point {
	if len(route) == 0 {
		return nil
	}
	out := make([]Point, 0, len(route))
	out = append(out, route[0])
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		d := HaversineMeters(a.Lat, a.Lng, b.Lat, b.Lng)
		steps := int(d / SampleStepMeters)
		for k := 1; k <= steps; k++ {
			r := float64(k) * SampleStepMeters / d
			out = append(out, Point{
				Lat: a.Lat + (b.Lat-a.Lat)*r,
				Lng: a.Lng + (b.Lng-a.Lng)*r,
			})
		}
		out = append(out, b)
	}
	return out
}

// MatchRatio devuelve la fracción (0..1) de samples dentro del radio de
// aceptación de CUALQUIER punto GPS del intento. Sin samples, sin radio, sin
// GPS o radio no positivo → 0 (D10: sin GPS suficiente → match % = 0).
func MatchRatio(samples []Point, radiusMeters float64, gps []Point) float64 {
	if len(samples) == 0 || len(gps) == 0 || radiusMeters <= 0 {
		return 0
	}
	inside := 0
	for _, s := range samples {
		for _, g := range gps {
			if HaversineMeters(s.Lat, s.Lng, g.Lat, g.Lng) <= radiusMeters {
				inside++
				break
			}
		}
	}
	return float64(inside) / float64(len(samples))
}

// MatchPercent es MatchRatio en porcentaje 0..100.
func MatchPercent(samples []Point, radiusMeters float64, gps []Point) float64 {
	return MatchRatio(samples, radiusMeters, gps) * 100
}
