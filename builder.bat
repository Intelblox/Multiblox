@echo off
if "%1" == "assets" GOTO assets
if "%1" == "build" GOTO build
if "%1" == "quick" GOTO quick
if "%1" == "clean" GOTO clean
exit /b 0

:assets
    go build -o .\cmd\MultibloxInstaller\assets -ldflags -H=windowsgui .\cmd\MultibloxPlayer
    go build -o .\cmd\MultibloxInstaller\assets .\cmd\Multiblox
exit /b 0

:build
    call :assets
    go build -o . .\cmd\MultibloxInstaller
exit /b 0

:quick
    call :assets
    copy /Y cmd\MultibloxInstaller\assets %userprofile%\AppData\Local\Multiblox
exit /b 0