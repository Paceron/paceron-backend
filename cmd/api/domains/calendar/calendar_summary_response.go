package calendar

// CalendarSummaryItem es un ítem de GET /users/{id}/calendar-summary y
// también de GET /sessions/{id}/assigned-groups (mismo shape, {group_id,group_name}).
type CalendarSummaryItem struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}
