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

// StorageClientInterface es el contrato del storage S3-compatible de Supabase.
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
}

type s3Client struct {
	client *s3.Client
	bucket string
}

// New arma un cliente S3 apuntando al endpoint S3-compatible de Supabase Storage
// (path-style, requerido por Supabase) con las credenciales de la stage resuelta.
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
		o.UsePathStyle = true
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

// PublicBaseURL deriva la URL pública base del bucket a partir del endpoint S3
// (formato `https://<project-ref>.storage.supabase.co/storage/v1/s3`) y el
// nombre del bucket. Supabase sirve objetos públicos desde un dominio distinto
// al del gateway S3 (`<project-ref>.supabase.co`, no `.storage.supabase.co`),
// así que no alcanza con reusar el endpoint tal cual. No requiere una env var
// extra: el project-ref ya está en el endpoint que se configura para el SDK S3.
func PublicBaseURL(endpoint, bucket string) string {
	projectRef := endpoint
	projectRef = strings.TrimPrefix(projectRef, "https://")
	projectRef = strings.TrimPrefix(projectRef, "http://")
	if idx := strings.Index(projectRef, "."); idx != -1 {
		projectRef = projectRef[:idx]
	}
	return fmt.Sprintf("https://%s.supabase.co/storage/v1/object/public/%s", projectRef, bucket)
}
