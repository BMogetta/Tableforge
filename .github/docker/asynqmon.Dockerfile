# Custom multi-arch Dockerfile for hibiken/asynqmon. Two reasons we don't
# use upstream's Dockerfile directly:
#
#  1. Upstream hardcodes `GOARCH=amd64` in the Go build stage, so even a
#     successful build targets amd64 only. The Pi cluster is arm64.
#  2. Upstream uses `alpine:3.17 + apk add nodejs npm` which pulls a node
#     version old enough that the React frontend's yarn build fails on
#     newer base images.
#
# Build context = the hibiken/asynqmon source tree (checked out by the
# workflow at .github/workflows/asynqmon-mirror.yml). buildx populates
# TARGETOS/TARGETARCH automatically based on --platform.

# ─── Frontend ────────────────────────────────────────────────────────────────
# Node 18 LTS — known to build the asynqmon ui/ tree without the
# openssl-legacy-provider hack that older Node versions need.
FROM --platform=$BUILDPLATFORM node:18-alpine AS frontend

WORKDIR /static

# Copy only the ui/ subtree to maximize cache reuse — backend changes
# alone won't invalidate this stage.
#
# We deliberately don't pass --frozen-lockfile here. yarn 1.x against
# Node 18 resolves a couple of transitive deps slightly differently
# than the lockfile that ships with hibiken/asynqmon (which was
# generated against an older Node). Strict lockfile fidelity isn't
# important when mirroring an external project — we catch real
# breakage at `yarn build`, not at install.
COPY ui/package.json ui/yarn.lock ./
RUN yarn install --network-timeout 600000

COPY ui/ ./
RUN yarn build

# ─── Backend ─────────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS backend

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Replace the upstream-bundled ui/build with our freshly compiled output.
COPY --from=frontend /static/build ./ui/build

ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

RUN go build -ldflags="-s -w" -o asynqmon ./cmd/asynqmon

# ─── Final ───────────────────────────────────────────────────────────────────
FROM scratch

COPY --from=backend /build/asynqmon /

ENTRYPOINT ["/asynqmon"]
