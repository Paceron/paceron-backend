package trainingplan

// Location es el shape {lat,lng,label?} compartido entre PlanDay.DefaultLocation
// (este paquete) y GroupCalendarDay.PresencialLocation (change de calendario).
type Location struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label *string `json:"label,omitempty"`
}
