@echo off
:: Replay a raw .pcapng capture: drag-and-drop a .pcapng file onto this bat,
:: or run: replay.bat <file.pcapng>
:: Starts the meter in file mode, re-parses everything and opens the browser.
if "%~1"=="" (
    echo Usage: drag a .pcapng file onto replay.bat, or: replay.bat ^<file.pcapng^>
    pause
    exit /b 1
)

:: Keep working directory at this folder so packet_log output lands here
cd /d "%~dp0"
"%~dp0dilmeterapi.exe" "%~1"
