package constants

type PlanDayKind string

const (
	PlanDayKindRest     PlanDayKind = "rest"
	PlanDayKindOther    PlanDayKind = "other"
	PlanDayKindTraining PlanDayKind = "training"
)

func GetValidPlanDayKinds() []string {
	return []string{
		string(PlanDayKindRest),
		string(PlanDayKindOther),
		string(PlanDayKindTraining),
	}
}

func IsValidPlanDayKind(kind string) bool {
	for _, k := range GetValidPlanDayKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
