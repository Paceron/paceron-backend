package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeRelativePath_BlocksTraversal(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"traversal con barra inicial", "/../../../../etc/passwd"},
		{"traversal sin barra inicial", "../../../../etc/passwd"},
		{"traversal mixto con plantuml", "/plantuml/../../../../etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeRelativePath(tc.input)
			assert.False(t, strings.Contains(got, ".."), "el resultado no debe contener '..' que escape de docsDir: %s", got)
			assert.True(t, strings.HasPrefix(got, "/"), "el resultado debe quedar enraizado: %s", got)
		})
	}
}

func TestSanitizeRelativePath_KeepsNormalPaths(t *testing.T) {
	assert.Equal(t, "/index.html", sanitizeRelativePath("/index.html"))
	assert.Equal(t, "/diagrams/foo.puml", sanitizeRelativePath("/diagrams/foo.puml"))
}
