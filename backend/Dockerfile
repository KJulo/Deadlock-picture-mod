# syntax=docker/dockerfile:1
# API image. Build from the REPO ROOT (the game files in templates/ are copied in):
#   docker build -f backend/Dockerfile -t catmod-api .
# The image contains Valve's files: push it only to a PRIVATE registry, never to a public one.

# --- 1. compile: official Go image (has gcc/g++ for cgo) -------------------------------------
FROM golang:1.27.3-trixie AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Fully static binary: glibc, libstdc++ (bc7decomp) and Go's pure-Go DNS/user lookups (netgo,
# osusergo), so the final image needs no system libraries at all.
RUN CGO_ENABLED=1 go build -trimpath -tags netgo,osusergo \
      -ldflags '-s -w -linkmode external -extldflags "-static"' \
      -o /out/server ./cmd/server

# --- 2. run: distroless (no shell, no package manager), non-root user --------------------------
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/server /app/server
COPY templates/ /app/templates/
ENV CATMOD_TEMPLATES=/app/templates \
    PORT=8080
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
