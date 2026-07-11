FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/demo-go ./cmd/demo-go

FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/demo-go /app/demo-go

USER app
EXPOSE 8000

ENTRYPOINT ["/app/demo-go"]
