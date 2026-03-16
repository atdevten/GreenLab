FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/supporting/ services/supporting/
WORKDIR /workspace/services/supporting
RUN go build -o /supporting ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /supporting /supporting
EXPOSE 8007
ENTRYPOINT ["/supporting"]
