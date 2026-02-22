### Developer Notes
- Do not attempt to build on Windows.  Use WSL2 where everything works.
  - On Windows there is a broken go-gl dependency that the fyne GUI library needs

- You will need docker-desktop.

- This is a standard `golang` program
  - install the go environment for your platform
  - run `go mod download` to get the dependencies

- VS Code works fine
  - Standard Go extensions
  - GitHub Free CoPilot works fine

- for Linux testing, use the terminal window in VS Code
  ```
  go run .
  ```
  or, to create a binary
  ```
  go build -o owlcms .
  ```

- On WSL2 you can use fyne-cross to generate the pi and Windows binaries for testing

  - install fyne-cross

     ```
     go install github.com/fyne-io/fyne-cross@latest
     ```
  
  - Cross-compile
     ```
      fyne-cross windows --app-id app.owlcms.controlpanel -name owlcms
      fyne-cross linux -arch arm64 --app-id app.owlcms.controlpanel -name owlcms-pi
      fyne-cross linux -arch amd64 --app-id app.owlcms.controlpanel -name owlcms-linux

### Releasing

See [RELEASE_PROCESS.md](RELEASE_PROCESS.md) for the release process.

