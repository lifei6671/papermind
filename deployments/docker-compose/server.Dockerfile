FROM golang:1.26-alpine AS build

WORKDIR /src

RUN apk add --no-cache build-base

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN go build -tags json1 -o /out/papermind ./cmd/papermind

FROM alpine:3.22

WORKDIR /app

RUN addgroup -S papermind && adduser -S papermind -G papermind \
  && mkdir -p /var/lib/papermind/tmp /var/lib/papermind/imports /var/lib/papermind/exports \
  && chown -R papermind:papermind /var/lib/papermind

COPY --from=build /out/papermind /app/papermind
COPY server/conf/app.example.yaml /app/conf/app.yaml
COPY server/data/migrations /app/data/migrations

USER papermind

EXPOSE 9080

CMD ["/app/papermind"]
