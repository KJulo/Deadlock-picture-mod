#!/usr/bin/env bash
# One-time Google Cloud setup. Run it yourself (it changes your cloud account):
#   GCP_PROJECT=my-project-id backend/deploy/setup-gcp.sh
# Needs: gcloud logged in (gcloud auth login) and billing linked to the project.
set -euo pipefail
: "${GCP_PROJECT:?set GCP_PROJECT to your project id}"
REGION="${GCP_REGION:-us-central1}"
REPO="${GCP_REPO:-catmod}"

echo "==> project $GCP_PROJECT, region $REGION"
gcloud config set project "$GCP_PROJECT"

echo "==> enabling Cloud Run and Artifact Registry"
gcloud services enable run.googleapis.com artifactregistry.googleapis.com

echo "==> private Docker repository '$REPO' (the image contains the game files: keep it private)"
if ! gcloud artifacts repositories describe "$REPO" --location "$REGION" >/dev/null 2>&1; then
  gcloud artifacts repositories create "$REPO" --repository-format docker --location "$REGION" \
    --description "catmod API images (private)"
fi

echo "==> keep only the 3 newest images (Artifact Registry free tier is 0.5 GB)"
cat > /tmp/catmod-cleanup.json <<'JSON'
[{"name": "keep-3", "action": {"type": "Keep"}, "mostRecentVersions": {"keepCount": 3}},
 {"name": "delete-old", "action": {"type": "Delete"}, "condition": {"olderThan": "1d"}}]
JSON
gcloud artifacts repositories set-cleanup-policies "$REPO" --location "$REGION" \
  --policy /tmp/catmod-cleanup.json --no-dry-run

echo "==> letting docker push to $REGION-docker.pkg.dev"
gcloud auth configure-docker "$REGION-docker.pkg.dev" --quiet

echo "done. Next: GCP_PROJECT=$GCP_PROJECT make deploy"
