package constants

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusPause     UserStatus = "pause"
	UserStatusBlocked   UserStatus = "blocked"
	UserStatusSuspended UserStatus = "suspended"
)

func GetValidUserStatuses() []string {
	return []string{
		string(UserStatusActive),
		string(UserStatusInactive),
		string(UserStatusPause),
		string(UserStatusBlocked),
		string(UserStatusSuspended),
	}
}

func IsValidUserStatus(status string) bool {
	for _, s := range GetValidUserStatuses() {
		if s == status {
			return true
		}
	}
	return false
}
