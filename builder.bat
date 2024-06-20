@echo off
if "%1" == "assets" GOTO assets
if "%1" == "build" GOTO build
if "%1" == "clean" GOTO clean
if "%1" == "quick" GOTO quick
exit /b 0

:assets
    mkdir cmd\MbxInstaller\assets
    copy /Y assets\icon.ico cmd\MbxInstaller\assets
    copy /Y cmd\Mbx\commands.txt cmd\MbxInstaller\assets
    copy /Y cmd\Mbx\roblox cmd\MbxInstaller\assets
    copy /Y cmd\Mbx\uninstall.bat cmd\MbxInstaller\assets
    go build -o .\cmd\MbxInstaller\assets -ldflags -H=windowsgui .\cmd\MbxPlayer
    go build -o .\cmd\MbxInstaller\assets .\cmd\Mbx
exit /b 0

:clean
    rmdir /S /Q cmd\MbxInstaller\assets
exit /b 0

:build
    call :assets
    go build -o . .\cmd\MbxInstaller
    call :clean
exit /b 0

:quick
    call :assets
    xcopy cmd\MbxInstaller\assets %userprofile%\AppData\Local\Multiblox /S /E /Y
    call :clean
exit /b 0