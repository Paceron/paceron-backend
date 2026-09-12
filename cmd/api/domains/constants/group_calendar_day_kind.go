package constants

// GroupCalendarDayKind extiende PlanDayKind con un 4to valor, cancelled,
// que no tiene sentido en un template (solo existe en el calendario real).
type GroupCalendarDayKind string

const (
	GroupCalendarDayKindRest      GroupCalendarDayKind = "rest"
	GroupCalendarDayKindOther     GroupCalendarDayKind = "other"
	GroupCalendarDayKindTraining  GroupCalendarDayKind = "training"
	GroupCalendarDayKindCancelled GroupCalendarDayKind = "cancelled"
)

func GetValidGroupCalendarDayKinds() []string {
	return []string{
		string(GroupCalendarDayKindRest),
		string(GroupCalendarDayKindOther),
		string(GroupCalendarDayKindTraining),
		string(GroupCalendarDayKindCancelled),
	}
}

func IsValidGroupCalendarDayKind(kind string) bool {
	for _, k := range GetValidGroupCalendarDayKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
