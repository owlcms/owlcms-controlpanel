Owlcms-launcher is a control panel for owlcms.  The control panel is used to

- Start and Stop owlcms

- Install updates
- Have multiple versions at once for testing purposes, with the ability to copy data from one to the other.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

### Change Log for version _TAG_

- Added a link to open owlcms on a localhost browser if it closed by mistake
- A configuration file is now available (`env.properties`) that allows setting any of the owlcms environment variables.  This can be used to change the port, load testing data, set features, etc.
- Added Help menu with link to documentation and about with version number
- More robust logic for downloading and using the correct Java runtime
- Added a File menu to allow removing the OWLCMS versions, the Java runtime, or the whole local directory
- Prevent multiple instances from being started, added a menu to kill left-over instance if the prevention failed.

### Installing the Control Panel

- For Windows, 
  1. Download `owlcms.exe` -- You may get warnings about rarely downloaded files, select the options to keep the file and download anyway.
  2. Drag or copy/paste the file to your Desktop
  3. Double-click on the owlcms executable you copied to your Desktop.
  4. If warnings are given, Use the "More Information"  and "Run Anyway" options to allow execution (only needed once)
  
- For Mac
  1. For a recent Mac (M1/M2/...), download the file that ends with `_arm64.dmg`.   For an Intel mac, use the `_intel.dmg` file
  2. Execute the `.dmg` file.  Drag the application to the Application folder, or drag the application to your desktop
  3. **Right-click on the application and use Open**.  This is needed once, to allow execution.

- For Raspberry Pi
  1. Download the file that ends with `_pi.deb` .
  2. Go to your Downloads directory, right-click on the downloaded file and select `Package Install`.  
     You will be prompted for your password to get installation permissions. This will create a desktop icon, and an entry in the "Other" section of the system menu.

- For Linux on Intel or AMD computers
  1. Download the file that ends with `_amd64.deb` .
  2. Start a terminal window and go to `~/Downloads`
  5. Use `sudo apt install ./name.deb` (replacing name with the actual name of the .deb file)

If you do not wish to use the package install, or lack privileges to do so, you may also copy the owlcms-pi or owlcms-linux binary file directly to your machine, but then there will be no icons.

### Running OWLCMS using the Control Panel

- Double-click on the icon on your desktop
  - This will download the latest version of OWLCMS and a Java runtime if there is none installed
  - Starting the program takes 20 to 30 seconds, and a browser window should open
  - A Stop button will be shown so you can politely stop the program.
  - If you stop the program, all the browsers will stay open and wait for a restart, so they must be stopped individually.