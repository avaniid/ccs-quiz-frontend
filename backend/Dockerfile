FROM golang:1.24.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o quiz-backend main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 ccs
WORKDIR /app
COPY --from=builder /app/quiz-backend .
COPY admin_portal/templates ./admin_portal/templates
COPY admin_portal/static admin_portal/static
USER ccs
ENV GIN_MODE=release
ENV PATH="/usr/local/bin:$PATH"
EXPOSE 2117
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:2117/health >/dev/null 2>&1 || exit 1
CMD ["./quiz-backend"]
