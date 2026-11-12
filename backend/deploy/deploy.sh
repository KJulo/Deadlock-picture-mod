#!/usr/bin/env bash
# Build the API image, push it to the private registry and deploy it to Cloud Run.
#   GCP_PROJECT=my-project-id ALLOWED_ORIGINS=https://catmod.pages.dev make deploy
set -euo pipefail
: "${GCP_PROJECT:?set GCP_PROJECT to your project id}"
REGION="${GCP_REGION:-us-central1}"
REPO="${GCP_REPO:-catmod}"
SERVICE="${GCP_SERVICE:-catmod-api}"
ORIGINS="${ALLOWED_ORIGINS:-http://localhost:4321}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

[ -f "$ROOT/templates/models/props_gameplay/soul_container/soul_container.vmdl_c" ] ||
  { echo "templates/ is empty: run 'make fetch' first"; exit 1; }

TAG="$(git -C "$ROOT" rev-parse --short HEAD)$(git -C "$ROOT" diff --quiet || echo -dirty)"
IMAGE="$REGION-docker.pkg.dev/$GCP_PROJECT/$REPO/api:$TAG"

echo "==> building $IMAGE"
if docker buildx version >/dev/null 2>&1; then
  docker build --platform linux/amd64 -f "$ROOT/backend/Dockerfile" -t "$IMAGE" "$ROOT"
else
  DOCKER_BUILDKIT=0 docker build -f "$ROOT/backend/Dockerfile" -t "$IMAGE" "$ROOT"
fi

echo "==> pushing"
docker push "$IMAGE"

# Flags, and why:
#   --cpu 2 --memory 1Gi      a build uses every core; 2 vCPU builds in ~2 s
#   --concurrency 4           up to 4 requests per instance (2 build, 2 wait in the app's queue)
#   --max-instances 2         hard cap: limits the bill if someone hammers the API
#   --min-instances 0         scale to zero when idle -> free when nobody uses it
#   --cpu-boost               extra CPU while starting -> faster cold starts
#   --allow-unauthenticated   it is a public website API
echo "==> deploying $SERVICE"
gcloud run deploy "$SERVICE" --project "$GCP_PROJECT" --region "$REGION" --image "$IMAGE" \
  --cpu 2 --memory 1Gi --concurrency 4 --max-instances 2 --min-instances 0 --cpu-boost \
  --timeout 120 --allow-unauthenticated \
  --set-env-vars "^@^ALLOWED_ORIGINS=$ORIGINS@MAX_CONCURRENT_BUILDS=2@MAX_UPLOAD_MB=15"

URL="$(gcloud run services describe "$SERVICE" --project "$GCP_PROJECT" --region "$REGION" --format 'value(status.url)')"
echo "==> $URL"
curl -fsS "$URL/healthz" && echo
