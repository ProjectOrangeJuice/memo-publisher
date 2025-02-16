FROM golang:1.22.2 AS build

WORKDIR /build
COPY ./src /build

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o syncer .

FROM ubuntu:latest

RUN apt update; apt install git -y

WORKDIR /app
COPY --from=build /build/syncer /app/syncer
ENTRYPOINT [ "/app/syncer" ]