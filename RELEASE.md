Owlcms-launcher is a control panel for owlcms.  The control panel is used to

- Start and Stop owlcms

- Install updates
- Have multiple versions at once for testing purposes, with the ability to copy data from one to the other.

The control panel is installed once. It will automatically download the current version of owlcms and the correct version of Java when used for the first time.

### Change Log for version _TAG_

- 1.8.0: Added an installer for Windows
- 1.8.0: Renamed the installers for better understandability

### Installing the Control Panel

> When downloading the following files, some browsers may give you warnings about "rarely downloaded files".   You may have to select "Keep" one or more times to download the file.

- For Windows, 
  1. Download the Windows installer from the Assets section below
  2. Use Open File to run the Installer, or go to your Downloads area and run it.
     - You may still get warnings about Windows protecting you. Use the "More Information"  and "Run Anyway" options to allow execution (only needed once)
  3. After installation, there will be an Icon on your Desktop, and an entry in the start menu.
- For Mac
  1. For a recent Mac (M1/M2/...), download the `.dmg`  file that is starts with `macOS_Apple`  
     For an older Intel mac, download the `macOS_Intel` file
  2. Execute the `.dmg` file.  Drag the application to the Application folder, or drag the application to your desktop
  3. The first time you use the program, you **must** *Right-click on the application and use Open.*  This is only needed once, to allow execution.
- For Raspberry Pi
  1. Download the `.deb` file that starts with `Raspberry`.
  2. Go to your Downloads directory, right-click on the downloaded file and select `Package Install`.  
     You will be prompted for your password to get installation permissions. This will create a desktop icon, and an entry in the "Other" section of the system menu.
- For Linux on Intel or AMD computers
  1. Download the `.deb` file that starts with `Linux_Intel`
  2. Start a terminal window and go to `~/Downloads`
  5. Use `sudo apt install ./Linux*.deb` (replacing the name with the actual name of the .deb file)

If you do not wish to use the package install, or lack privileges to do so, you may also copy the owlcms-pi or owlcms-linux binary file directly to your machine, but then there will be no icons.

### Using the Control Panel

- See the instructions at https://owlcms.github.io/owlcms4/#/LocalControlPanel