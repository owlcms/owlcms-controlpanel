This is a control panel for owlcms.  It is meant to:

- Start and Stop owlcms
- Install updates
- Allow for multiple installations to be present at once for testing purposes, with the ability to copy data from one to another.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

### Change Log
- 3.0.1-rc02: Restore compatibility with macOS 13 and 14
- 3.0.1-rc01: Fixed false "You are not connected to the internet" messages
- 3.0.1-rc01: Added capability to install Tracker packages from zip, for custom plugins
- 3.0.0-rc12: zip package is now device-independent
- 3.0.0-rc11: Fix the semantic version value of the control panel sent to launched programs
- 3.0.0-rc09: Show a dialog if "OWLCMS Ready" does not show up before 60 seconds with a link to the logs.
- 3.0.0-rc09: Added "Duplicate" and "Rename" functions to make copies of an application

### New in version 3.0
- Control owlcms, owlcms-tracker and owlcms-firmata from the same control panel
- Show a dialog if "OWLCMS Ready" does not show up before 60 seconds with a link to the logs.
- Added "Duplicate" and "Rename" functions to make copies of an application
- Tail a startup log for OWLCMS version 64
- Consolidation and cleanup of Java and Node runtimes
- Improved import process

### Installing the Control Panel

> When downloading the following files, some browsers may give you warnings about "rarely downloaded files".   You may have to select "Keep" one or more times to download the file.

- For Windows, 
  1. Download the Windows installer from the Assets section below.
  2. Drag the file from the Downloads area to your Desktop. 
  3. The first time you run the program, you may get warnings in a blue dialog box.  Select "More Info" and "Run Anyway"
  4. NOTE: if the installer cannot be run (sometimes antivirus software falsely detects it), you can use the `.exe` file directly instead.
- For Mac
  1. Download the macOS `.dmg`  file
  4. Execute the `.dmg` file.  Drag the application to the Application folder, or drag the application to your desktop
  5. The first time you use the program, you **must** *Right-click on the application and use Open.*  This is only needed once, to allow execution.
- For Raspberry Pi
  1. Download the `.deb` file that starts with `Raspberry`.
  2. Go to your Downloads directory, *right-click on the downloaded file and select `Package Install`.*
     You will be prompted for your password to get installation permissions. This will create a desktop icon, and an entry in the "Other" section of the system menu.
- For Linux on Intel or AMD computers
  1. Download the `.deb` file that starts with `Linux_Intel`
  2. Start a terminal window and go to `~/Downloads`
  6. Use `sudo apt install ./Linux*.deb` (replacing the name with the actual name of the .deb file)

If you do not wish to use the package install, or lack privileges to do so, you may also copy the owlcms-pi or owlcms-linux binary file directly to your machine, but then there will be no icons.       

### Using the Control Panel
