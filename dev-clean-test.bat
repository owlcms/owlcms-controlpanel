@echo off
setlocal enabledelayedexpansion
REM Full clean test: scrub configs, ffmpeg, versions, rebuild everything, deploy, launch.
REM Run from the owlcms-controlpanel directory.

set "PANEL_DIR=%APPDATA%\owlcms-controlpanel"
set "CAMERAS_DIR=%APPDATA%\owlcms-cameras"
set "REPLAYS_DIR=%APPDATA%\owlcms-replays"
set "REPLAYS_SRC=%USERPROFILE%\git\replays"
set "PANEL_SRC=%~dp0"
REM Strip trailing backslash from PANEL_SRC
if "%PANEL_SRC:~-1%"=="\" set "PANEL_SRC=%PANEL_SRC:~0,-1%"

set "DEV_VERSION=0.0.0-dev"

echo === 1. Scrubbing installed state ===

echo   Removing video_config (ffmpeg.toml, shared configs)...
if exist "%PANEL_DIR%\video_config" rd /s /q "%PANEL_DIR%\video_config"

echo   Removing downloaded ffmpeg...
if exist "%PANEL_DIR%\ffmpeg" rd /s /q "%PANEL_DIR%\ffmpeg"

echo   Removing cameras versions...
if exist "%CAMERAS_DIR%" rd /s /q "%CAMERAS_DIR%"

echo   Removing replays versions...
if exist "%REPLAYS_DIR%" rd /s /q "%REPLAYS_DIR%"

echo.
echo === 2. Building replays binaries ===
pushd "%REPLAYS_SRC%"
if errorlevel 1 (
    echo ERROR: Cannot cd to %REPLAYS_SRC%
    exit /b 1
)

echo   Building cameras_windows.exe...
go build -o cameras_windows.exe ./cmd/cameras
if errorlevel 1 (
    echo ERROR: cameras build failed
    popd
    exit /b 1
)

echo   Building replays_windows.exe...
go build -o replays_windows.exe ./cmd/replays
if errorlevel 1 (
    echo ERROR: replays build failed
    popd
    exit /b 1
)
popd

echo.
echo === 3. Building control panel ===
pushd "%PANEL_SRC%"
go build -o controlpanel.exe .
if errorlevel 1 (
    echo ERROR: control panel build failed
    popd
    exit /b 1
)
popd

echo.
echo === 4. Creating version dirs and deploying dev binaries ===

set "CAMERAS_VER_DIR=%CAMERAS_DIR%\%DEV_VERSION%"
set "REPLAYS_VER_DIR=%REPLAYS_DIR%\%DEV_VERSION%"

if not exist "%CAMERAS_VER_DIR%" mkdir "%CAMERAS_VER_DIR%"
if not exist "%REPLAYS_VER_DIR%" mkdir "%REPLAYS_VER_DIR%"

copy /y "%REPLAYS_SRC%\cameras_windows.exe" "%CAMERAS_VER_DIR%\cameras_windows.exe" >nul
copy /y "%REPLAYS_SRC%\replays_windows.exe" "%REPLAYS_VER_DIR%\replays_windows.exe" >nul

echo   Deployed cameras to %CAMERAS_VER_DIR%\
echo   Deployed replays to %REPLAYS_VER_DIR%\

echo.
echo === 5. Launching control panel ===
echo   (ffmpeg download, config extraction, and launch will happen from the UI)
echo.

"%PANEL_SRC%\controlpanel.exe"
