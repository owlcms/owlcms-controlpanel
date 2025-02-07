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
  go build -o firmata .
  ```

- On WSL2 you can use fyne-cross to generate the pi and Windows binaries for testing

  - install fyne-cross

     ```
     go install github.com/fyne-io/fyne-cross@latest
     ```
  
  - Cross-compile
     ```
     fyne-cross windows --app-id app.owlcmx.firmata-launcher -name firmata
     fyne-cross linux -arch arm64 --app-id app.owlcmx.firmata-launcher -name firmata-pi
     fyne-cross linux -arch amd64 --app-id app.owlcmx.firmata-launcher -name firmata-linux

### Releasing
- decide on a tag number
- update the version number in release.sh to match
- update RELEASE.md
- commit and push all the files
- trigger the github actions workflow by creating the tag you decided above and pushing it.
  For example
  
  ```
  git tag v1.5.2-alpha13 && git push origin --tags
  ```
- once the workflow has finished run release to finish the work.
  This will eventually be fixed by extending the github actions workflow with a second job
  
  ```
  ./release.sh
  ```

