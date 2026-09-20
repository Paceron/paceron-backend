package calendar

// CalendarSummaryItem es un ítem de GET /users/{id}/calendar-summary.
type CalendarSummaryItem struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}
