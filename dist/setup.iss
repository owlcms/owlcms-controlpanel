[Setup]
AppName=firmata Control Panel
AppVersion=1.9.3400
AppPublisher=Jean-François Lamy
AppPublisherURL=https://firmata.jflamy.dev
AppSupportURL=https://groups.google.com/g/firmata
AppUpdatesURL=https://github.com/owlcms-firmata/firmata-controlpanel
VersionInfoVersion=1.9.3400
VersionInfoCompany=firmata
VersionInfoDescription=firmata Control Panel Installer
VersionInfoCopyright=© 2024 Jean-François Lamy
MinVersion=10.0
Compression=lzma2/ultra64
SolidCompression=yes
DefaultDirName={userappdata}\firmata Control Panel
OutputDir=.
OutputBaseFilename=firmata-Panel-installer_windows
SetupIconFile=installer.ico
UninstallDisplayIcon={app}\installer.ico
PrivilegesRequired=lowest
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableStartupPrompt=yes

[Files]
Source: "iss\firmata.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "iss\firmata.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "iss\installer.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\firmata Control Panel"; Filename: "{app}\firmata.exe"; IconFilename: "{app}\firmata.ico"
Name: "{userdesktop}\firmata Control Panel"; Filename: "{app}\firmata.exe"; IconFilename: "{app}\firmata.ico"
Name: "{group}\Uninstall firmata Control Panel"; Filename: "{uninstallexe}"; IconFilename: "{app}\installer.ico"

[Run]
Filename: "{app}\firmata.exe"; Description: "Launch firmata Control Panel"; Flags: nowait postinstall skipifsilent
