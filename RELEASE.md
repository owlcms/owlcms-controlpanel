## Release Process

### Steps to Release

1. Decide on a tag number
2. Update the version number in `release.sh` to match the desired tag
3. Update `ReleaseNotes.md` with the changes for this release
4. Run the release script:
   
   ```bash
   ./release.sh
   ```
   
   This script will:
   - Clean up any existing tag/release with the same version
   - Pull latest changes
   - Update build configuration
   - Commit and push all changes (including your version/release notes updates)
   - Create and push the tag, which automatically triggers the GitHub Actions workflow

5. The GitHub Actions workflow will automatically build all platforms and create the release

*Note: The workflow is triggered automatically when the tag is pushed, so no manual workflow triggering is needed.*
