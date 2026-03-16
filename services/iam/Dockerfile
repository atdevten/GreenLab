FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/iam/ services/iam/
WORKDIR /workspace/services/iam
RUN go build -o /identity ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /identity /identity
EXPOSE 8001
ENTRYPOINT ["/identity"]
