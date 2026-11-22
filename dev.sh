#!/bin/sh
# Runs the API against the local Postgres and SeaweedFS S3 (docker compose -f docker-compose.dev.yml up -d).
# Login codes are printed in this terminal because SMTP is not configured.
cd "$(dirname "$0")/api"
export DATABASE_URL=postgres://cotinha:cotinha@127.0.0.1:5434/cotinha
export S3_ENDPOINT=127.0.0.1:8333 S3_ACCESS_KEY=cotinha S3_SECRET_KEY=cotinha-dev-secret
export S3_BUCKET=cotinha S3_USE_SSL=false S3_REGION=us-east-1 S3_CREATE_BUCKET=true
export ADDR=127.0.0.1:8080 PUBLIC_URL=http://localhost:5173
exec go run .
