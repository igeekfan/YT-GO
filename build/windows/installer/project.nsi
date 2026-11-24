Unicode true

!include "wails_tools.nsh"
!include "MUI.nsh"
!include "WinCore.nsh"
!include "Win\WinNT.nsh"
!include "Win\WinError.nsh"

!define MOVEFILE_REPLACE_EXISTING 1

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"
ManifestDPIAware true

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show

Var UpdatePID
Var UpdateFile

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

!define MUI_PAGE_CUSTOMFUNCTION_PRE SkipUpdatePage
!insertmacro MUI_PAGE_WELCOME
!define MUI_PAGE_CUSTOMFUNCTION_PRE SkipUpdatePage
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

LangString UpdateWait ${LANG_ENGLISH} "Waiting for YT-GO to exit..."
LangString UpdateWait ${LANG_SIMPCHINESE} "正在等待 YT-GO 退出……"
LangString UpdateWaitFailed ${LANG_ENGLISH} "YT-GO did not exit. Close the app before trying the update again."
LangString UpdateWaitFailed ${LANG_SIMPCHINESE} "YT-GO 尚未退出。请关闭应用后重试更新。"
LangString UpdateReplaceFailed ${LANG_ENGLISH} "Could not replace $INSTDIR\${PRODUCT_EXECUTABLE}."
LangString UpdateReplaceFailed ${LANG_SIMPCHINESE} "无法替换 $INSTDIR\${PRODUCT_EXECUTABLE}。"

Function .onInit
    !insertmacro wails.checkArchitecture
    ${GetParameters} $0
    ${GetOptions} $0 "/UPDATEPID=" $UpdatePID
    ClearErrors
FunctionEnd

Function SkipUpdatePage
    ${If} $UpdatePID != ""
        Abort
    ${EndIf}
FunctionEnd

Function WaitForOldProcess
    DetailPrint "$(UpdateWait)"
    System::Call 'kernel32::OpenProcess(i ${SYNCHRONIZE}, i 0, i $UpdatePID) p.r0 ?e'
    Pop $1
    ${If} $0 != 0
        System::Call 'kernel32::WaitForSingleObject(p r0, i 60000) i.r1'
        System::Call 'kernel32::CloseHandle(p r0)'
        ${If} $1 == ${WAIT_OBJECT_0}
            Return
        ${EndIf}
    ${ElseIf} $1 == ${ERROR_INVALID_PARAMETER}
        Return
    ${EndIf}

    SetErrorLevel 3
    Abort "$(UpdateWaitFailed)"
FunctionEnd

Function ApplyUpdate
    Call WaitForOldProcess

    IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 failed
    ClearErrors
    GetTempFileName $UpdateFile $INSTDIR
    IfErrors failed

    SetOverwrite try
    !ifdef SUPPORTS_AMD64
        ${If} ${IsNativeAMD64}
            File "/oname=$UpdateFile" "${ARG_WAILS_AMD64_BINARY}"
        ${EndIf}
    !endif
    !ifdef SUPPORTS_ARM64
        ${If} ${IsNativeARM64}
            File "/oname=$UpdateFile" "${ARG_WAILS_ARM64_BINARY}"
        ${EndIf}
    !endif
    SetOverwrite on
    IfErrors removeStage
    System::Call 'kernel32::MoveFileExW(w "$UpdateFile", w "$INSTDIR\${PRODUCT_EXECUTABLE}", i ${MOVEFILE_REPLACE_EXISTING}) i.r0'
    ${If} $0 == 0
        Goto removeStage
    ${EndIf}

    SetRegView 64
    ReadRegStr $0 SHCTX "${UNINST_KEY}" "DisplayIcon"
    ${If} $0 == "$INSTDIR\${PRODUCT_EXECUTABLE}"
        WriteRegStr SHCTX "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    ${EndIf}
    ClearErrors
    Return

removeStage:
    Delete "$UpdateFile"
failed:
    SetErrorLevel 4
    Abort "$(UpdateReplaceFailed)"
FunctionEnd

Section
    !insertmacro wails.setShellContext
    ${If} $UpdatePID != ""
        Call ApplyUpdate
    ${Else}
        !insertmacro wails.webview2runtime

        SetOutPath $INSTDIR
        !insertmacro wails.files

        CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
        CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

        !insertmacro wails.associateFiles
        !insertmacro wails.associateCustomProtocols
        !insertmacro wails.writeUninstaller
    ${EndIf}
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols
    !insertmacro wails.deleteUninstaller
SectionEnd
