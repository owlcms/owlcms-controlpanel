### Developer Notes
- Do not attempt to build on Windows.  Use WSL2 where everything works.
  - On Windows there is a broken go-gl dependency that the fyne GUI library needs
- You will need Docker (Docker-desktop on Windows is fine).
- Standard `golang` program
  - install the go environment for your platform
  - `go mod download` to get the dependencies
- VS Code works fine
  - Standard Go extensions
  - GitHub Free CoPilot works fine
- Use fyne-cross to generate the pi and Windows binary.
  ```
  fyne-cross windows --app-id app.owlcms.owlcms-launcher -name owlcms-windows
  fyne-cross linux -arch arm64 --app-id app.owlcms.owlcms-launcher -name owlcms-pi
  fyne-cross linux -arch amd64 --app-id app.owlcms.owlcms-launcher -name owlcms-linux
  ```
  
### Releasing
- update the version number in release.sh
- update RELEASE.md
- run ./release.sh

### To be explored
  -  download and extract Federation-specific overrides
