# syntax=docker/dockerfile:1

FROM golang:1.21

RUN apt update -y

WORKDIR /var/www/html/app/back
ENV GO111MODULE=on

COPY /app/back/go.mod ./
RUN go mod download
RUN go mod verify

RUN go install -mod=mod github.com/githubnemo/CompileDaemon


EXPOSE 8084