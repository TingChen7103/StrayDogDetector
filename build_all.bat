@echo off
setlocal

:: 設定路徑變數
set FRONT_DIR=front
set FRONT_DIST=front\dist
set GO_STATIC_DIR=cmd\dilmeterapi\static
set GO_MAIN_DIR=./cmd/dilmeterapi
set OUTPUT_EXE=dilmeterapi.exe

echo ==========================================
echo [1/4] Checking Frontend Dependencies...
echo ==========================================
cd %FRONT_DIR%

:: 檢查 node_modules 是否存在，若不存在代表是第一次執行，需要安裝依賴
if not exist "node_modules" (
    echo [INFO] node_modules not found. Installing dependencies...
    call npm install
    if %errorlevel% neq 0 goto :error
) else (
    echo [INFO] Dependencies already installed.
)

echo.
echo ==========================================
echo [2/4] Building Frontend (Vue)...
echo ==========================================
:: 執行 Vue 編譯
call npm run build
if %errorlevel% neq 0 goto :error
cd ..

echo.
echo ==========================================
echo [3/4] Update Static Assets...
echo ==========================================
:: 清理並重建 static 資料夾
if exist "%GO_STATIC_DIR%" (
    rmdir /s /q "%GO_STATIC_DIR%"
)
mkdir "%GO_STATIC_DIR%"

:: 複製 dist 內容
xcopy /s /e /y "%FRONT_DIST%\*.*" "%GO_STATIC_DIR%\"
if %errorlevel% neq 0 goto :error

echo.
echo ==========================================
echo [4/4] Building Go Binary...
echo ==========================================
:: 執行 Go build
go build -o %OUTPUT_EXE% %GO_MAIN_DIR%
if %errorlevel% neq 0 goto :error

echo.
echo ==========================================
echo [SUCCESS] Build Complete! Output: %OUTPUT_EXE%
echo ==========================================
pause
exit /b 0

:error
echo.
echo ==========================================
echo [ERROR] Build Failed! Please check logs.
echo ==========================================
pause
exit /b %errorlevel%