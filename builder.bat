@echo off
if "%1" == "assets" GOTO assets
if "%1" == "build" GOTO build
if "%1" == "quick" GOTO quick
if "%1" == "clean" GOTO clean
exit /b 0

:assets
    go build -o .\cmd\MbxInstaller\assets -ldflags -H=windowsgui .\cmd\MbxPlayer
    go build -o .\cmd\MbxInstaller\assets .\cmd\Mbx
exit /b 0

:build
    call :assets
    go build -o . .\cmd\MbxInstaller
exit /b 0

:quick
    call :assets
    xcopy cmd\MbxInstaller\assets %userprofile%\AppData\Local\Multiblox /S /E /Y
exit /b 0