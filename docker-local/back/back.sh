#!/bin/bash
crontab -
CompileDaemon --build="go build cmd/main/main.go" --command=./main
