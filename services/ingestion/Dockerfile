FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/ingestion/ services/ingestion/
WORKDIR /workspace/services/ingestion
RUN go build -o /ingestion ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /ingestion /ingestion
EXPOSE 8003
ENTRYPOINT ["/ingestion"]
