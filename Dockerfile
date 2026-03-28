FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY main.go ./
COPY web ./web
RUN go build -o /out/wakey .

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=build /out/wakey /usr/local/bin/wakey
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/wakey"]
