Owlcms-launcher is a control panel for owlcms.  It replaces the prior installation methods used until version 54 of owlcms.

- You will be able to start and stop owlcms from the control panel.  
- You will be able to install updates
- It will automatically download the current version of owlcms when used for the first time.
- It will get the correct version of Java when launching owlcms for the first time
- You can also have several versions at once. You can copy your database and local changes between versions

### Change Log for version _TAG_

- More robust logic for downloading and using the correct Java runtime
- Added a File menu to allow removing OWLCMS versions, or the Java runtime, or the whole directory (including config files etc.)
- Added a File menu entry to kill a Java process started by another instance of the control panel.
- Updated the DMG background in the futile hope of centering the arrow.

### Installing the Launcher

- For Windows, 
  1. Download `owlcms.exe`  
  2. Drag or copy/paste the file to your Desktop
  3. Double-click on the owlcms.exe file. If warnings are given, Use the "More Information"  and "Run Anyway" options to allow execution (only needed once)
  
- For Mac
  1. For a recent Mac (M1/M2/...), download the file that ends with `_arm64.dmg`.   For an Intel mac, use the `_amd.dmg` file
  2. Execute the `.dmg` file.  Drag the application to the Application folder.
  3. **Right-click on the application and use Open**.  This is needed once, to allow execution.

- For Raspberry Pi
  1. Download the file that ends with `_pi.deb` .
  2. Go to your Downloads directory, right-click on the downloaded file and select `Package Install`.  
     You will be prompted for the password of your pi user. This will create a desktop icon, and an entry in the "Other" section of the system menu.

- For Linux on Intel or AMD computers
  1. Download the file that ends with `_amd64.deb` .
  2. Start a terminal window and go to `~/Downloads`
  5. Use `sudo apt install ./name.deb` (replacing name with the actual name of the .deb file)

If you do not wish to use the package install, or lack privileges to do so, you may also copy the owlcms-pi or owlcms-linux binary file directly to your machine, but then there will be no icons.

### Running OWLCMS using the Launcher

- Double-click on the icon on your desktop
  - This will download the latest version of OWLCMS if there is none installed
  - When launching for the first time, the launcher will fetch an appropriate version of the Java programming language
  - Starting the program takes 20 to 30 seconds, and a browser window should open
  - A Stop button will be shown so you can politely stop the program.
  - If you stop the program, all the browsers will stay open and wait for a restart, so they must be stopped individually.