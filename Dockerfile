FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ARG APP_NAME
COPY ${APP_NAME} /app/app
RUN mkdir -p /app/tmp /app/data
ENTRYPOINT ["/app/app"]
