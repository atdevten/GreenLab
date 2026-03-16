FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/normalization/ services/normalization/
WORKDIR /workspace/services/normalization
RUN go build -o /normalization ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /normalization /normalization
EXPOSE 8006
ENTRYPOINT ["/normalization"]
