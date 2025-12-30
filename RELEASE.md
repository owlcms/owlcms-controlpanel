This is a control panel for owlcms.  It is meant to:

- Start and Stop owlcms
- Install updates
- Allow for multiple installations to be present at once for testing purposes, with the ability to copy data from one to another.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

> BEWARE MacOS users: This version requires MacOS 15 or later to run.  Support for building for earlier versions has been removed from the
> build environment used (GitHub).  If you need support for version 13 and 14 you will need to keep using version 2.7.

### Change Log
- 3.0.0-rc02: resolve differences in behavior between build and dev versions
- 3.0.0: Major Release
  - The control panel now controls owlcms, owlcms-tracker and owlcms-firmata from the same application (jury replays will come later)
  - You can therefore run, if you need to, more than one application on the same machine, from the same panel.

### Previous versions
- 2.8.0: When installing from a zip, 
  - a prefix like "owlcms-" will be stripped, 
  - the timestamp produced by "Save Installed Version as zip" will be stripped
  - if extracting the same version multiple times, the directories will have 01 02 03 etc. added
- 2.8.0: Revised import process. Local is now reset to match the current jar, and then all additions,
deletions and modifications done in the imported release relative to the imported release's jar
are applied.  This will make it easier for federations to provide customized kits.
- 2.8.0: When using the "update" button on a version, that version is kept as is, as a backup.

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

- See the instructions at https://owlcms.github.io/owlcms4/#/LocalControlPanel