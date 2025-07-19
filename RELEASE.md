This is a control panel for owlcms.  It is meant to:

- Start and Stop owlcms
- Install updates
- Have multiple versions at once for testing purposes, with the ability to copy data from one to the other.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

### Change Log

- 2.4.3: Fix for Raspberry Pi Java version - the latest JDK build is missing 2/3 of the expected releases, using a predetermined version instead.
- 2.4.3: Make the Java version fetched configurable instead of being "latest".  Can be overridden as TEMURIN_VERSION in env.properties
- 2.4.2: Show a link to the configuration files of the running owlcms version while it is running
- 2.4.1: Fix Raspberry Pi Desktop file to not require an execution confirmation
- 2.4.0: Ability to install from a downloaded zip, making it easier for federations to prepare a kit with their own templates and settings
- 2.4.0: The update procedure now preserves additions, changes and deletions made in the version being updated.
- 2.4.0: Don't warn users about control panel updates that are prerelease
- 2.4.0: Accept any valid semver version as a directory, allowing 57.1.0+federation as a valid name with metadata
- 2.4.0: On Raspberry Pi, the desktop icon no longer prompts for execution.

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