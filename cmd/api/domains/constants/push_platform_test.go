package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushPlatformValues(t *testing.T) {
	assert.Equal(t, PushPlatform("android"), PushPlatformAndroid)
	assert.Equal(t, PushPlatform("web"), PushPlatformWeb)
}

func TestGetValidPushPlatforms(t *testing.T) {
	platforms := GetValidPushPlatforms()
	expected := []string{"android", "web"}
	assert.Equal(t, expected, platforms)
	assert.Len(t, platforms, 2)
}

func TestIsValidPushPlatform_Valid(t *testing.T) {
	assert.True(t, IsValidPushPlatform("android"))
	assert.True(t, IsValidPushPlatform("web"))
}

func TestIsValidPushPlatform_Invalid(t *testing.T) {
	assert.False(t, IsValidPushPlatform(""))
	assert.False(t, IsValidPushPlatform("ios"))
	assert.False(t, IsValidPushPlatform("Android"))
}
