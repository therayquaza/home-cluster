FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

WORKDIR /app

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pix main.go

FROM scratch

WORKDIR /root/

COPY --from=builder /app/pix .

COPY assets/ ./assets/

EXPOSE 8080

CMD ["./pix"]