package storageclient

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StorageClientInterface es el contrato del storage S3-compatible.
// Sin lógica de negocio: no conoce usuarios, equipos, ni reglas de validación —
// eso vive en el service que lo consume.
type StorageClientInterface interface {
	Upload(ctx context.Context, key string, content []byte, contentType string) error
	Delete(ctx context.Context, key string) error
}

type Options struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	// ForcePathStyle direcciona como host/bucket/key en vez de bucket.host/key.
	// Ambos backends que usa el proyecto (Supabase Storage y el storage local del
	// docker-compose) necesitan path-style. El default lo pone config
	// (S3_FORCE_PATH_STYLE, default true); el zero value de Go es false, así que
	// un New() armado a mano tiene que pasarlo explícitamente.
	ForcePathStyle bool
	// PublicBaseURL overridea la URL pública base del bucket. Vacío = se deriva
	// del endpoint (ver PublicBaseURL).
	PublicBaseURL string
}

type s3Client struct {
	client *s3.Client
	bucket string
}

// New arma un cliente S3 apuntando al endpoint configurado para el stage
// resuelto (Supabase Storage en testing/producción, el storage local del
// docker-compose con --stage=local). Path-style por default.
func New(ctx context.Context, opts Options) (StorageClientInterface, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(opts.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading AWS config for storage client: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(opts.Endpoint)
		o.UsePathStyle = opts.ForcePathStyle
	})

	return &s3Client{client: client, bucket: opts.Bucket}, nil
}

func (c *s3Client) Upload(ctx context.Context, key string, content []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("error uploading object %q: %w", key, err)
	}
	return nil
}

func (c *s3Client) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error deleting object %q: %w", key, err)
	}
	return nil
}

// PublicBaseURL devuelve la URL pública base del bucket a partir del endpoint S3
// y el nombre del bucket, para armar las URLs que el frontend carga sin
// credenciales (avatar, ícono de equipo).
//
// El override gana si viene seteado: el derivado de abajo es específico de
// Supabase y no aplica a ningún otro S3, así que el stage local pasa su propia
// base (S3_PUBLIC_BASE_URL, típicamente http://localhost:9000/<bucket>).
//
// Sin override, el derivado asume la forma de Supabase: de
// `https://<project-ref>.storage.supabase.co/storage/v1/s3` sale
// `https://<project-ref>.supabase.co/storage/v1/object/public/<bucket>`, porque
// Supabase sirve los objetos públicos desde otro dominio que el gateway S3, así
// que no alcanza con reusar el endpoint. No requiere env var extra: el
// project-ref ya está en el endpoint que se configura para el SDK.
func PublicBaseURL(endpoint, bucket, override string) string {
	if override != "" {
		return strings.TrimSuffix(override, "/")
	}

	projectRef := endpoint
	projectRef = strings.TrimPrefix(projectRef, "https://")
	projectRef = strings.TrimPrefix(projectRef, "http://")
	if idx := strings.Index(projectRef, "."); idx != -1 {
		projectRef = projectRef[:idx]
	}
	return fmt.Sprintf("https://%s.supabase.co/storage/v1/object/public/%s", projectRef, bucket)
}
