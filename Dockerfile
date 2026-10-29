FROM golang AS build

WORKDIR /build
COPY ./ /build

RUN CGO_ENABLED=0 GOOS=linux go build -o /build/app cmd/httpserver/main.go

FROM scratch

USER 1000

WORKDIR /app
COPY --from=build /build/app /app/app

ENTRYPOINT ["/app/app"]
