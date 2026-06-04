FROM gcr.io/distroless/static:nonroot
WORKDIR /app
ARG APP_NAME
COPY --chown=65532:65532 ${APP_NAME} /app/app
ENTRYPOINT ["/app/app"]
