@echo off
setlocal
REM Apps-only dev helper: reset app versions, build cameras/replays, deploy dev binaries.
REM Leaves control panel files and binary untouched.

set "CAMERAS_DIR=%APPDATA%\owlcms-cameras"
set "REPLAYS_DIR=%APPDATA%\owlcms-replays"
set "GIT_DIR=C:\Dev\git"
set "REPLAYS_SRC=%GIT_DIR%\replays"
set "DEV_VERSION=0.0.0-dev"

echo === 1. Resetting app installed state ===

echo   Removing cameras versions...
if exist "%CAMERAS_DIR%" rd /s /q "%CAMERAS_DIR%"

echo   Removing replays versions...
if exist "%REPLAYS_DIR%" rd /s /q "%REPLAYS_DIR%"

echo.
echo === 2. Building app binaries ===
pushd "%REPLAYS_SRC%"
if errorlevel 1 (
	echo ERROR: Cannot cd to %REPLAYS_SRC%
	exit /b 1
)

echo   Building cameras_windows.exe...
go build -buildvcs=false -o cameras_windows.exe ./cmd/cameras
if errorlevel 1 (
	echo ERROR: cameras build failed
	popd
	exit /b 1
)

echo   Building replays_windows.exe...
go build -buildvcs=false -o replays_windows.exe ./cmd/replays
if errorlevel 1 (
	echo ERROR: replays build failed
	popd
	exit /b 1
)
popd

echo.
echo === 3. Deploying app binaries ===
set "CAMERAS_VER_DIR=%CAMERAS_DIR%\%DEV_VERSION%"
set "REPLAYS_VER_DIR=%REPLAYS_DIR%\%DEV_VERSION%"

if not exist "%CAMERAS_VER_DIR%" mkdir "%CAMERAS_VER_DIR%"
if not exist "%REPLAYS_VER_DIR%" mkdir "%REPLAYS_VER_DIR%"

copy /y "%REPLAYS_SRC%\cameras_windows.exe" "%CAMERAS_VER_DIR%\cameras_windows.exe" >nul
copy /y "%REPLAYS_SRC%\replays_windows.exe" "%REPLAYS_VER_DIR%\replays_windows.exe" >nul

echo   Deployed cameras to %CAMERAS_VER_DIR%\
echo   Deployed replays to %REPLAYS_VER_DIR%\

echo.
echo Apps-only reset/build/deploy complete.
exit /b 0
