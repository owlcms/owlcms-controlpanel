## Release Process

### Steps to Release

1. Decide on a tag number
2. Update the version number in `release.sh` to match
3. Update `ReleaseNotes.md` with the changes for this release
4. Commit and push all the files
5. Trigger the GitHub Actions workflow by creating the tag and pushing it:
   
   ```bash
   git tag v1.5.2-alpha13 && git push origin --tags
   ```

6. Once the workflow has finished, run the release script to finish the work:
   
   ```bash
   ./release.sh
   ```
   
   *Note: This will eventually be fixed by extending the GitHub Actions workflow with a second job*
