#!/bin/bash

# Update and upgrade package list
sudo apt update
sudo apt upgrade -y

# Enable 32-bit architecture (required for Wine)
sudo dpkg --add-architecture i386
sudo apt update

# Install Wine
sudo apt install -y wine64 wine32

# Verify Wine installation
wine --version

# Download Inno Setup installer locally
wget https://jrsoftware.org/download.php/is.exe -O is.exe

# Run Inno Setup installer with Wine
wine is.exe /SILENT

# Create Inno Setup script (setup.iss)
cat <<EOL > setup.iss
[Setup]
AppName=My Application
AppVersion=1.0
DefaultDirName={userappdata}\\My Application
OutputDir=.
OutputBaseFilename=MyAppInstaller
SetupIconFile=path/to/your/icon.ico
PrivilegesRequired=lowest

[Files]
Source: "path/to/your/files/*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\\My Application"; Filename: "{app}\\YourExecutable.exe"; IconFilename: "{app}\\path/to/your/icon.ico"
Name: "{userdesktop}\\My Application"; Filename: "{app}\\YourExecutable.exe"; IconFilename: "{app}\\path/to/your/icon.ico"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "startmenushortcut"; Description: "{cm:CreateStartMenuIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Code]
procedure InitializeWizard;
begin
  WizardForm.Components[1].Visible := False;  // Hide the Welcome page
  WizardForm.Components[4].Visible := False;  // Hide the License page
  WizardForm.Components[5].Visible := False;  // Hide the Information page
  WizardForm.Components[6].Visible := False;  // Hide the Components page
  WizardForm.Components[7].Visible := False;  // Hide the Tasks page
  WizardForm.Components[8].Visible := False;  // Hide the Ready page
  WizardForm.Components[9].Visible := False;  // Hide the Installing page
  WizardForm.Components[10].Visible := False; // Hide the Finished page
  
  WizardForm.TasksList.Checked[0] := True;    // Check the Desktop Icon checkbox by default
  WizardForm.TasksList.Checked[1] := True;    // Check the Start Menu Shortcut checkbox by default
end;
EOL

# Run Inno Setup compiler with Wine
wine "C:\\Program Files (x86)\\Inno Setup 6\\ISCC.exe" setup.iss

echo "Inno Setup installer creation process completed."