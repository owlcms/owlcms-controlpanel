### Developer Notes
- Native development builds can work on Windows, Linux, and other supported Go platforms.
- Windows Fyne builds can still be noticeably slower than Linux builds.
- WSL2 is not required for normal development builds.
- Some Fyne tooling, especially `fyne-cross`, may still work better or require WSL2 plus Docker Desktop for packaging and cross-platform bundle creation.

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

- VS Code target settings
  - The default [.vscode/settings.json](.vscode/settings.json) in this repo is set up for Linux arm64 analysis.
  - When switching development targets, copy the matching section into [.vscode/settings.json](.vscode/settings.json).
  - Linux arm64:
    ```json
    "go.toolsEnvVars": {
        "GOOS": "linux",
        "GOARCH": "arm64"
    },
    "gopls": {
        "build.env": {
            "GOOS": "linux",
            "GOARCH": "arm64"
        }
    }
    ```
  - Windows amd64:
    ```json
    "go.toolsEnvVars": {
        "GOOS": "windows",
        "GOARCH": "amd64"
    },
    "gopls": {
        "build.env": {
            "GOOS": "windows",
            "GOARCH": "amd64"
        }
    }
    ```

- `fyne-cross` can be used to generate the pi and Windows binaries for testing
  - depending on your host setup, this may require WSL2 and Docker Desktop

  - install fyne-cross

     ```
     go install github.com/fyne-io/fyne-cross@latest
     ```
  
  - Cross-compile
     ```
      fyne-cross windows --app-id app.owlcms.controlpanel -name owlcms
      fyne-cross linux -arch arm64 --app-id app.owlcms.controlpanel -name owlcms-pi
      fyne-cross linux -arch amd64 --app-id app.owlcms.controlpanel -name owlcms-linux
     ```

- Windows local build notes
  - For a normal Windows build with console output, use:
    ```
    go build -buildvcs=false -o controlpanel.exe .
    ```
  - This is better while debugging because stdout, stderr, and traces remain visible.
  - For a Windows GUI-style executable without a console window, use:
    ```
    go build -buildvcs=false -ldflags="-H windowsgui" -o controlpanel.exe .
    ```

### Releasing

See [RELEASE_PROCESS.md](RELEASE_PROCESS.md) for the release process.

