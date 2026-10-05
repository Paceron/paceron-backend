#!/usr/bin/env bash
# Inicializa el bucket de media en el storage S3-compatible local (RustFS).
# Lo corre el servicio `storage-init` del docker-compose al arrancar, no a mano.
#
# Hace dos cosas, ambas idempotentes:
#   1. crea el bucket si no existe
#   2. lo deja legible sin credenciales (GetObject público)
#
# El punto 2 no es opcional: en Supabase el bucket `paceron-media` está creado
# con `public = true`, y el backend arma URLs públicas de avatar/ícono de equipo
# que el frontend carga SIN mandarle credenciales (ver
# storageclient.PublicBaseURL). Si el bucket local no es público, esas URLs dan
# 403 y se ve como un bug de la app en vez de como config del entorno.
#
# Variables de entorno (las inyecta el compose):
#   S3_ENDPOINT, S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY,
#   AWS_DEFAULT_REGION
set -euo pipefail

: "${S3_ENDPOINT:?falta S3_ENDPOINT}"
: "${S3_BUCKET:?falta S3_BUCKET}"

aws_s3() {
  aws --endpoint-url "$S3_ENDPOINT" "$@"
}

if aws_s3 s3api head-bucket --bucket "$S3_BUCKET" >/dev/null 2>&1; then
  echo "bucket $S3_BUCKET ya existía"
else
  aws_s3 s3api create-bucket --bucket "$S3_BUCKET"
  echo "bucket $S3_BUCKET creado"
fi

# Public Access Block en off es requisito previo para que una bucket policy de
# solo lectura pública sea efectiva en la mayoría de los S3 compatibles.
aws_s3 s3api put-public-access-block \
  --bucket "$S3_BUCKET" \
  --public-access-block-configuration \
  BlockPublicAcls=false,IgnorePublicAcls=false,RestrictPublicBuckets=false >/dev/null

# Solo lectura. Nada de escritura ni borrado público: el backend siempre sube y
# borra con credenciales, el público es solo para que el browser pueda pintar
# la imagen.
policy=$(printf '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}' "$S3_BUCKET")

aws_s3 s3api put-bucket-policy --bucket "$S3_BUCKET" --policy "$policy" >/dev/null

aws_s3 s3api head-bucket --bucket "$S3_BUCKET" >/dev/null
echo "bucket $S3_BUCKET listo (lectura pública habilitada)"