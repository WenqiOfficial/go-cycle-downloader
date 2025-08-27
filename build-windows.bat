@echo off
cd  %~dp0

go mod download

go mod tidy

go build -o build/windows .

@pause