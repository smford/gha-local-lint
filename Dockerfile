FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/gha-local-lint ./cmd/gha-local-lint

FROM alpine:3.20

RUN apk --no-cache add ca-certificates git

COPY --from=builder /bin/gha-local-lint /usr/local/bin/gha-local-lint

ENTRYPOINT ["/usr/local/bin/gha-local-lint"]
CMD ["check"]
