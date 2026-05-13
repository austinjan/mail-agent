@echo off
setlocal

if not exist bin mkdir bin

echo Building mail-agent...
go build -o bin\mail-agent.exe .\cmd\mail-agent
if errorlevel 1 (
    echo Build failed.
    pause
    exit /b 1
)

echo Build completed: bin\mail-agent.exe
pause
