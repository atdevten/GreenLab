FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/alert-notification/ services/alert-notification/
WORKDIR /workspace/services/alert-notification
RUN go build -o /alert-notification ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /alert-notification /alert-notification
EXPOSE 8005
ENTRYPOINT ["/alert-notification"]
