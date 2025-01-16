[Setup]
AppName=owlcms Control Panel
AppVersion=1.0
DefaultDirName={userappdata}\owlcms Control Panel
OutputDir=.
OutputBaseFilename=owlcms-Panel-installer_windows
SetupIconFile=installer.ico
UninstallDisplayIcon={app}\installer.ico
PrivilegesRequired=lowest
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableStartupPrompt=yes

[Files]
Source: "iss\owlcms.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "iss\owlcms.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "iss\installer.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\owlcms Control Panel"; Filename: "{app}\owlcms-launcher.exe"; IconFilename: "{app}\owlcms.ico"
Name: "{userdesktop}\owlcms Control Panel"; Filename: "{app}\owlcms-launcher.exe"; IconFilename: "{app}\owlcms.ico"
Name: "{group}\Uninstall owlcms Control Panel"; Filename: "{uninstallexe}"; IconFilename: "{app}\installer.ico"

[Run]
Filename: "{app}\owlcms-launcher.exe"; Description: "Launch owlcms Control Panel"; Flags: nowait postinstall skipifsilent