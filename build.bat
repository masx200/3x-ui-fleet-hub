@echo off
setlocal enabledelayedexpansion

REM 3X-UI Frontend Build Script for Windows

echo ================================================
echo 3X-UI Fleet Hub - Frontend Build Script
echo ================================================
echo.

if "%1"=="clean" goto clean
if "%1"=="help" goto help
if "%1"=="frontend" goto frontend_only
if "%1"=="" goto full_build
if "%1"=="all" goto full_build

:help
echo Usage:
echo   build.bat          - Build frontend and Go binary (default)
echo   build.bat all      - Build frontend and Go binary
echo   build.bat frontend - Build frontend only
echo   build.bat clean    - Clean build artifacts
echo   build.bat help     - Show this help message
goto end

:clean
echo → Cleaning build artifacts...
if exist web\build rmdir /s /q web\build
if exist web\node_modules rmdir /s /q web\node_modules
if exist bin rmdir /s /q bin
echo ✓ Clean completed
goto end

:frontend_only
echo → Building frontend...
cd web
if not exist node_modules (
    echo → Installing frontend dependencies...
    call npm install
    if errorlevel 1 (
        echo ✗ npm install failed
        cd ..
        goto end
    )
)
call npm run build
if errorlevel 1 (
    echo ✗ Frontend build failed
    cd ..
    goto end
)
cd ..
echo.
echo ================================================
echo ✓ Frontend build completed!
echo ================================================
goto end

:full_build
echo → Building frontend and Go binary...
cd web
if not exist node_modules (
    echo → Installing frontend dependencies...
    call npm install
    if errorlevel 1 (
        echo ✗ npm install failed
        cd ..
        goto end
    )
)
call npm run build
if errorlevel 1 (
    echo ✗ Frontend build failed
    cd ..
    goto end
)
cd ..

echo → Building Go binary...
if not exist bin mkdir bin
go build -o bin\3x-ui.exe main.go
if errorlevel 1 (
    echo ✗ Go build failed
    goto end
)

echo.
echo ================================================
echo ✓ Production build completed successfully!
echo ================================================
echo   Binary: bin\3x-ui.exe
echo.

:end
endlocal
