Owlcms-launcher is a control panel for owlcms.  The control panel is used to

- Start and Stop owlcms
- Install updates
- Have multiple versions at once for testing purposes, with the ability to copy data from one to the other.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

### Change Log

- 1.9.4-alpha: MSIX installer, no changes to control panel.
- 1.9.3: Single installer for macOS.
- 1.9.2: Ask for confirmation if closing the window while owlcms is running since this will stop owlcms
- 1.9.2: Removed the Windows installer, due to virus false detection on Windows.  Instructions for owlcms now refer to using the executable directly.
- 1.9.1: Now copy the windows executable as well as the installer
- 1.9.1: The build process is now conditional for the various platforms
- 1.9.0: The `env.properties` file is now reloaded before launching owlcms, at every launch
- 1.9.0: Added a "Check for Updates" entry in the help menu.

### Installing the Control Panel

> When downloading the following files, some browsers may give you warnings about "rarely downloaded files".   You may have to select "Keep" one or more times to download the file.

- For Windows, 
  1. Download the Windows `owlcms_controlpanel.exe` executable from the Assets section below
  2. Drag the file from the Downloads area to your Desktop. 
  3. The first time you run the program, you may get warnings in a blue dialog box.  Select "More Info" and "Run Anyway"
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

- See the instructions at https://owlcms.github.io/owlcms4/#/LocalControlPanel