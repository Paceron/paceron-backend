package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecret(t *testing.T) {
	assert.Equal(t, "", MaskSecret(""))
	assert.Equal(t, "***", MaskSecret("short"))
	assert.Equal(t, "****ijkl(len=12)", MaskSecret("abcdefghijkl"))
	assert.Equal(t, "****7890(len=15)", MaskSecret("TEST-1234567890"))
	assert.NotContains(t, MaskSecret("TEST-1234567890"), "TEST-123")
	assert.NotContains(t, MaskSecret("TEST-1234567890"), "12345")
}
