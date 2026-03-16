FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/query-realtime/ services/query-realtime/
WORKDIR /workspace/services/query-realtime
RUN go build -o /query-realtime ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /query-realtime /query-realtime
EXPOSE 8004
ENTRYPOINT ["/query-realtime"]
