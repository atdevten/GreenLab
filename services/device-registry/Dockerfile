FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/device-registry/ services/device-registry/
WORKDIR /workspace/services/device-registry
RUN go build -o /device-registry ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /device-registry /device-registry
EXPOSE 8002
ENTRYPOINT ["/device-registry"]
