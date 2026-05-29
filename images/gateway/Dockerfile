# Gateway proxy image
# Builds the transparent egress proxy from source
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/gateway

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /gateway /usr/local/bin/gateway

# Gateway listens on port 8080
EXPOSE 8080

ENTRYPOINT ["gateway"]
