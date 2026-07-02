FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct
ENV GOSUMDB=off

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o news .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/news .
COPY --from=builder /app/tmpl ./tmpl
COPY --from=builder /app/static ./static

RUN mkdir -p /app/data

EXPOSE 8080
VOLUME /app/data

ENV DB_PATH=/app/data/news.db
ENV PORT=8080
ENV TZ=Asia/Shanghai

CMD ["./news"]