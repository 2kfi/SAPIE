Name "SAPIE for Windows"
OutFile "${OUT}"
InstallDir "$LOCALAPPDATA\SAPIE"
RequestExecutionLevel user
SetCompressor /SOLID lzma
Icon "${ICON}"
UninstallIcon "${ICON}"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "SAPIE for Windows"
VIAddVersionKey "FileDescription" "SAPIE for Windows installer"
VIAddVersionKey "CompanyName" "SAPIE"
VIAddVersionKey "LegalCopyright" "SAPIE contributors. Non-commercial."
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"

Section "Install"
  SetOutPath "$INSTDIR"
  File /r "${SOURCE}\*.*"
  File "${ICON}"

  CreateShortCut "$SMPROGRAMS\SAPIE\SAPIE for Windows.lnk" "$INSTDIR\SAPIE.exe" "" "$INSTDIR\sapie.ico" 0

  WriteRegStr HKCU "Software\Classes\sapie" "" "URL:SAPIE protocol"
  WriteRegStr HKCU "Software\Classes\sapie" "URL Protocol" ""
  WriteRegStr HKCU "Software\Classes\sapie\DefaultIcon" "" "$INSTDIR\SAPIE.exe,0"
  WriteRegStr HKCU "Software\Classes\sapie\shell\open\command" "" '"$INSTDIR\SAPIE.exe" "%1"'

  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher" "DisplayName" "SAPIE for Windows"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher" "DisplayIcon" "$INSTDIR\sapie.ico"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher" "Publisher" "SAPIE"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher" "DisplayVersion" "${VERSION}"

  WriteUninstaller "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Uninstall"
  MessageBox MB_YESNO "Delete your SAPIE library record? Game files are never deleted." /SD IDNO IDYES keep_library_delete IDNO keep_library_move

  keep_library_delete:
    Delete "$INSTDIR\library.json"
    Goto remove_shortcut

  keep_library_move:
    IfFileExists "$INSTDIR\library.json" 0 remove_shortcut
    Delete "$LOCALAPPDATA\SAPIE-library.json"
    Rename "$INSTDIR\library.json" "$LOCALAPPDATA\SAPIE-library.json"

  remove_shortcut:
    Delete "$SMPROGRAMS\SAPIE\SAPIE for Windows.lnk"
    RMDir "$SMPROGRAMS\SAPIE"
    DeleteRegKey HKCU "Software\Classes\sapie"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SAPIE.Launcher"
    RMDir /r "$INSTDIR"
SectionEnd
