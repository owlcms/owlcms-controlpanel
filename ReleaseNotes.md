This is a control panel for owlcms and associated modules.  It is meant to:

- Start and Stop owlcms, owlcms-tracker and owlcms-firmata
- Install updates
- Allow for multiple installations to be present at once for testing purposes, with the ability to copy data from one to another.

The control panel is installed once. It will automatically download the correct version of Java when used for the first time.

## Release Log
- 3.0.5: The shared owlms/tracker/firmata control panel is now the official way to install and run owlcms-firmata
- 3.0.5: Copy env.properties when updating/importing owlcms and owlcms-firmata that have such a file

- 3.0.4: Accept non-Latin letters in the semantic versioning descriptive metadata (after the +), since metadata is ignored for version comparison anyway.
This deviation from the standard is so all the non-English countries can have proper descriptions when naming "Install from Zip" releases.

- 3.0.3: Fix a (harmless) error message when firmata for Arduino Devices was not installed.  The configuration file is now correctly created.
- 3.0.2: An env.properties file is created in each owlcms installation, initialized with the parent env.properties. This allows overriding environment variables per installation.
- 3.0.2: Under Windows, logging control-panel.log in the Control Panel installation directory should now work.
- 3.0.2: Factually accurate progress bar for the long unzip in tracker
- 3.0.2: Correctly deal with opening folders that have a + in their name

### Notable Changes for version 3.0.x
- Control owlcms, owlcms-tracker and owlcms-firmata from the same control panel
- Consolidation and cleanup of Java and Node runtimes
- Added "Duplicate" and "Rename" functions to make copies of an application
- Added capability to install Tracker packages from zip, for custom plugins
- Show a dialog if "OWLCMS Ready" does not show up before 60 seconds with a link to the logs.
- Improved import process for owlcms

### Installing the Control Panel

> When downloading the following files, some browsers may give you warnings about "rarely downloaded files".   You may have to select "Keep" one or more times to download the file.

- For Windows, 
  1. Download the Windows installer from the Assets section below.
  2. Execute the .exe file
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

