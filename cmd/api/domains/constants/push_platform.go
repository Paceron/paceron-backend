package constants

// PushPlatform identifica el tipo de dispositivo dueño de un push token — determina
// cómo se interpreta el token al enviarlo (hoy solo Android vía Expo; web queda
// reservado para cuando se implemente esa pila, ver openspec/changes/push-notifications).
type PushPlatform string

const (
	PushPlatformAndroid PushPlatform = "android"
	PushPlatformWeb     PushPlatform = "web"
)

func GetValidPushPlatforms() []string {
	return []string{
		string(PushPlatformAndroid),
		string(PushPlatformWeb),
	}
}

func IsValidPushPlatform(platform string) bool {
	for _, p := range GetValidPushPlatforms() {
		if p == platform {
			return true
		}
	}
	return false
}
