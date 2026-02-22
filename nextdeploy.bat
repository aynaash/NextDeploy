@echo off
setlocal

:: NextDeploy CLI Installer for Windows

echo Installing NextDeploy CLI for Windows...

:: Define download URL (update this once your release process is established)
set VERSION=latest
set DOWNLOAD_URL=https://github.com/NextDeploy/NextDeploy/releases/download/%VERSION%/nextdeploy-windows-amd64.exe
set INSTALL_DIR=%USERPROFILE%\.nextdeploy\bin
set EXECUTABLE=%INSTALL_DIR%\nextdeploy.exe

if not exist "%INSTALL_DIR%" (
    mkdir "%INSTALL_DIR%"
)

echo Downloading %DOWNLOAD_URL%...
curl -fLo "%EXECUTABLE%" "%DOWNLOAD_URL%"

if %errorlevel% neq 0 (
    echo Error downloading NextDeploy CLI.
    exit /b %errorlevel%
)

echo Adding to PATH...
setx PATH "%PATH%;%INSTALL_DIR%"

echo ✅ NextDeploy CLI installed successfully!
echo Please restart your terminal to use the 'nextdeploy' command.

endlocal
