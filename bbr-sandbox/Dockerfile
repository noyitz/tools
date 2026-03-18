FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bbr-sandbox .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /bbr-sandbox /bbr-sandbox
EXPOSE 8888
ENTRYPOINT ["/bbr-sandbox"]
