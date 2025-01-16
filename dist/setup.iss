[Setup]
AppName=owlcms Control Panel
AppVersion=1.0
DefaultDirName={userappdata}\owlcms Control Panel
OutputDir=.
OutputBaseFilename=owlcms-Windows-installer
SetupIconFile=installer.ico
UninstallDisplayIcon={app}\installer.ico
PrivilegesRequired=lowest
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableStartupPrompt=no

[Files]
Source: "..\fyne-cross\bin\windows-amd64\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "owlcms.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "installer.ico"; DestDir: "{app}"; Flags: ignoreversion
[Icons]
Name: "{group}\owlcms Control Panel"; Filename: "{app}\owlcms-launcher.exe"; IconFilename: "{app}\owlcms.ico"
Name: "{userdesktop}\owlcms Control Panel"; Filename: "{app}\owlcms-launcher.exe"; IconFilename: "{app}\owlcms.ico"
Name: "{group}\Uninstall owlcms Control Panel"; Filename: "{uninstallexe}"; IconFilename: "{app}\installer.ico"

[Run]
Filename: "{app}\owlcms-launcher.exe"; Description: "Launch owlcms Control Panel"; Flags: nowait postinstall skipifsilent