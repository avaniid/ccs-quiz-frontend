# --- Build Stage ---
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o quiz-backend main.go

# --- Run Stage ---
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/quiz-backend .
COPY admin_portal/templates ./admin_portal/templates
COPY admin_portal/static admin_portal/static

# No .env is copied: it is gitignored, so it does not exist in a CI/deploy
# build context and "COPY .env .env" fails the build there. Configuration comes
# from real environment variables in production (the host's env var settings);
# godotenv.Load() just logs and moves on when no file is present, so local
# development with a .env still works unchanged.

ENV PATH="/usr/local/bin:$PATH"

EXPOSE 2117

CMD ["./quiz-backend"]