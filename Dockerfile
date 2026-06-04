FROM gcr.io/distroless/static:nonroot
WORKDIR /app
ARG APP_NAME
COPY ${APP_NAME} /app/app
ENTRYPOINT ["/app/app"]
