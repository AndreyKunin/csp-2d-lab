# ── Stage 1: Frontend build ───────────────────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app/static

COPY static/package.json static/package-lock.json ./
RUN npm ci

COPY static/gulpfile.js static/tsconfig.json ./
COPY static/src ./src
COPY static/static ./static

RUN npm run build

# ── Stage 2: Go build ─────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY application.go application.yaml ./
COPY ga ./ga
COPY mvc ./mvc
COPY problem ./problem
COPY util ./util
COPY --from=frontend-builder /app/static/static ./static/static

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o csp2d .

# ── Stage 3: Minimal runtime ──────────────────────────────────────────────────
FROM scratch

WORKDIR /app

COPY --from=go-builder /app/csp2d ./csp2d
COPY --from=go-builder /app/application.yaml ./application.yaml
COPY --from=go-builder /app/static/static ./static

EXPOSE 8080

ENTRYPOINT ["/app/csp2d"]