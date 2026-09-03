package services

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/restclients/storageclient"
)

// buildMediaURL arma la URL pública de un objeto de storage a partir de su key
// y la fecha de última actualización (usada como parámetro de cache-busting).
// nil-safe: si key es nil (sin foto/ícono cargado), devuelve nil.
func buildMediaURL(key *string, updatedAt *time.Time) *string {
	if key == nil || *key == "" {
		return nil
	}

	version := int64(0)
	if updatedAt != nil {
		version = updatedAt.Unix()
	}

	base := storageclient.PublicBaseURL(config.MyStorage.Endpoint, config.MyStorage.Bucket)
	url := fmt.Sprintf("%s/%s?v=%d", base, *key, version)
	return &url
}

// MaxPhotoSizeBytes acota fotos de perfil/ícono de equipo — no es un caso de uso
// de archivos grandes, 5MB alcanza de sobra para una foto comprimida.
const MaxPhotoSizeBytes = 5 * 1024 * 1024

// allowedPhotoTypes mapea el content-type real (detectado por magic bytes, no por
// lo que declare el cliente) a la extensión usada en la key del bucket.
var allowedPhotoTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

var (
	ErrPhotoTooLarge    = errors.New("el archivo supera el tamaño máximo permitido (5MB)")
	ErrPhotoInvalidType = errors.New("tipo de archivo no permitido, solo se aceptan imágenes JPEG, PNG o WEBP")
)

// validatePhotoContent valida tamaño y tipo real (magic bytes) de una foto de
// perfil/ícono de equipo, y devuelve el content-type detectado y la extensión
// correspondiente para armar la key determinística.
func validatePhotoContent(content []byte) (contentType, ext string, err error) {
	if len(content) > MaxPhotoSizeBytes {
		return "", "", ErrPhotoTooLarge
	}

	detected := http.DetectContentType(content)
	ext, ok := allowedPhotoTypes[detected]
	if !ok {
		return "", "", ErrPhotoInvalidType
	}

	return detected, ext, nil
}
