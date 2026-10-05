package storageclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicBaseURL_WithOverride(t *testing.T) {
	// El stage local: el derivado de Supabase no aplica a un S3 local, asi que
	// S3_PUBLIC_BASE_URL gana siempre que venga.
	got := PublicBaseURL(
		"http://localhost:9000",
		"paceron-media",
		"http://localhost:9000/paceron-media",
	)
	assert.Equal(t, "http://localhost:9000/paceron-media", got)
}

func TestPublicBaseURL_OverrideIgnoresEndpoint(t *testing.T) {
	// Aunque el endpoint diga .supabase.co, con override el endpoint no se mira:
	// el override es la decision explicita.
	got := PublicBaseURL(
		"https://abc.supabase.co/storage/v1/s3",
		"paceron-media",
		"http://localhost:9000/paceron-media",
	)
	assert.Equal(t, "http://localhost:9000/paceron-media", got)
}

func TestPublicBaseURL_OverrideTrimsTrailingSlash(t *testing.T) {
	// Sin trim, el caller arma "base/key" y queda doble slash. URL de imagenes
	// con // suele andar igual, pero es una fuente de URLs distintas para el
	// mismo objeto y rompe el cache-busting.
	got := PublicBaseURL("http://localhost:9000", "paceron-media", "http://localhost:9000/paceron-media/")
	assert.Equal(t, "http://localhost:9000/paceron-media", got)
}

func TestPublicBaseURL_NoOverride_DerivesSupabase(t *testing.T) {
	// Sin override, comportamiento de siempre: project-ref del host del gateway
	// S3, y el dominio publico de Supabase (que no es el del gateway).
	got := PublicBaseURL("https://abcproject.storage.supabase.co/storage/v1/s3", "paceron-media", "")
	assert.Equal(t, "https://abcproject.supabase.co/storage/v1/object/public/paceron-media", got)
}

func TestPublicBaseURL_NoOverride_EndpointWithPath(t *testing.T) {
	// El project-ref sale de la parte de host; el path del endpoint no debe
	// filtrarse al resultado.
	got := PublicBaseURL("http://abcproject.storage.supabase.co:5432/storage/v1/s3", "paceron-media", "")
	assert.Equal(t, "https://abcproject.supabase.co/storage/v1/object/public/paceron-media", got)
}

func TestPublicBaseURL_NoOverride_EmptyOverrideFallsBack(t *testing.T) {
	// Una env var declarada pero vacia tiene que caer al derivado, no devolver "".
	// Si devolviera "", buildMediaURL armaria "/avatars/user-1.png" y el frontend
	// pediria una URL relativa.
	got := PublicBaseURL("https://abcproject.storage.supabase.co/storage/v1/s3", "paceron-media", "")
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "abcproject.supabase.co")
}

// New no toca la red, solo arma el cliente, asi que se puede testear el
// path-style sin nada escuchando.
func TestNew_ForcePathStyle(t *testing.T) {
	for _, forcePathStyle := range []bool{true, false} {
		client, err := New(t.Context(), Options{
			Endpoint:        "http://localhost:9000",
			Region:          "us-east-1",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			Bucket:          "paceron-media",
			ForcePathStyle:  forcePathStyle,
		})
		assert.NoError(t, err)
		assert.NotNil(t, client)

		s3c, ok := client.(*s3Client)
		assert.True(t, ok)
		assert.Equal(t, forcePathStyle, s3c.client.Options().UsePathStyle, "ForcePathStyle=%v", forcePathStyle)
		assert.Equal(t, "http://localhost:9000", *s3c.client.Options().BaseEndpoint)
	}
}

func TestNew_UploadsAndDeletesTargetConfiguredBucket(t *testing.T) {
	client, err := New(t.Context(), Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "otro-bucket",
		ForcePathStyle:  true,
	})
	assert.NoError(t, err)

	s3c, ok := client.(*s3Client)
	assert.True(t, ok)
	assert.Equal(t, "otro-bucket", s3c.bucket)
}
