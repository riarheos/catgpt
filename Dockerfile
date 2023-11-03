FROM        gcr.io/distroless/static-debian12
COPY        catgpt /app
ENTRYPOINT  ["/app"]
