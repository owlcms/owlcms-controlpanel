# Import Data and Config Process

## Overview
The import process migrates user customizations from a previous OWLCMS installation to a new version. It preserves database files and local configuration changes while ensuring the new version gets all its reference files from the new JAR.

## Process Flow

### Phase 1: Analysis
Analyze what the user changed in the old version relative to the old reference JAR.

1. **Build file inventories**
   - Extract checksums for all files from old `owlcms.jar` (reference state)
   - List all files in old `local/` directory
   - Sort both lists for parallel traversal

2. **Parallel traversal optimization**
   - Walk both file lists simultaneously in lexicographic order
   - Detect complete directory additions/deletions early
   - Skip checksum computation for entire directories when possible
   - Only compute checksums for files that exist in both (potential modifications)

4. **Classify changes**
   - **DELETED**: Files in old JAR but not in old local
   - **ADDED**: Files in old local but not in old JAR
     - If a directory existed in old JAR but has completely different files, individual files are marked as ADDED/DELETED rather than consolidating the directory
   - **MODIFIED**: Files with different checksums between old local and old JAR
   - **UNCHANGED**: Files with identical checksums

### Phase 2: Extract Fresh Reference from New JAR

1. **Extract fresh reference from new JAR**
   ```
   If newDir/local/ exists: Remove it
   Extract: newJar → newDir/local/
   Result: All reference files from new version are present
   ```

### Phase 3: Apply Deletions

2. **Delete files that user deleted in old version**
   ```
   For each file/dir in filesDeletedFromOldJar:
     Remove: newDir/local/{file or directory}
   Reason: Respect user's intention to remove these files
   ```

### Phase 4: Apply Modifications

3. **Restore user's modified files**
   ```
   For each file in filesModifiedInOldLocal:
     Copy: oldDir/local/{file} → newDir/local/{file}
   Reason: Preserve user customizations
   ```

### Phase 5: Apply Additions

4. **Add user's new files**
   ```
   For each file/dir in filesAddedToOldLocal:
     Copy: oldDir/local/{file or directory} → newDir/local/
   Reason: Bring over user additions
   Note: Overwrites if same file exists in new version
   ```

### Database Migration (Handled Separately)

Copy database files (already handled separately before calling this function):
```
Copy: oldDir/database/ → newDir/database/
```

## Result

After import completes, the new version will have:

| File Type | Result |
|-----------|--------|
| New files in new JAR | ✅ Present (unless user deleted in old version) |
| User's modified files | ✅ Applied (old version overwrites new) |
| User's added files/dirs | ✅ Present (overwrites new if name collision) |
| User's deleted files/dirs | ✅ Removed from new version |
| Unchanged files | ✅ Uses new version from JAR |

## Performance Optimizations

1. **Directory-level detection**
   - Entire directories added/deleted are identified early
   - Avoids computing checksums for hundreds of files in translation/template directories

2. **Checksum caching**
   - Checksums computed during analysis are cached
   - Reused during application phase if needed
   - Only files that exist in both trees get checksummed

3. **Parallel traversal**
   - Single pass through sorted file lists
   - O(n + m) complexity instead of O(n × m)

## Example Scenario

### Old Version (63.0.0)
- User deleted: `translations/de/` directory
- User added: `translations/custom/` directory with 50 files
- User modified: `local/application.properties`
- Unchanged: `templates/` directory (500 files)

### New Version (64.0.0)
- New JAR contains: Updated `templates/`, new `translations/fr/`
- No changes in local directory yet

### After Import
- ❌ `translations/de/` - Deleted (user removed it)
- ✅ `translations/fr/` - Present (new in 64.0.0)
- ✅ `translations/custom/` - Present (user's addition)
- ✅ `templates/` - All 500 files use new version from JAR
- ✅ `local/application.properties` - User's modified version

**Performance**: Only 1 checksum computed (application.properties), not 550+ files.

## Warning Messages

### Before Import Starts
```
⚠️ WARNING: Import Process

Any changes you made directly in version {newVersion} will be LOST.

This will:
1. Extract fresh local files from version {newVersion} (any changes you made directly in version {newVersion} will be LOST)
2. Apply the same additions, deletions and modifications you made in version {oldVersion}

Do you want to continue?
[Import] [Cancel]
```

### During Import
```
=== Import Analysis: User Changes Relative to Old owlcms.jar ===
Old owlcms.jar (reference): /path/to/63.0.0/owlcms.jar
Old local directory: /path/to/63.0.0/local

Files in old owlcms.jar: 450
Files in old local directory: 475

Files DELETED from old owlcms.jar: 2
Files DELETED from old owlcms.jar (in old jar but not in old local): 2
  - DELETED in old local: translations/de/file1.properties
  - DELETED in old local: translations/de/file2.properties

Files ADDED to old local: 50
  + ADDED to old local: translations/custom/file1.properties
  ... (49 more files)

Files MODIFIED in old local: 1
  * MODIFIED in old local: local/application.properties

Files UNCHANGED in old local: 447
=== End Import Analysis ===

Phase 2: Extracting fresh reference from new JAR...
  - Extracting local/ directory from new JAR...

Phase 3: Applying deletions (2 items)...
  - Deleting: translations/de/file1.properties
  - Deleting: translations/de/file2.properties

Phase 4: Applying modifications (1 files)...
  - Updating: local/application.properties

Phase 5: Applying additions (50 items)...
  + Adding: translations/custom/file1.properties
  ... (49 more files)

=== Import Complete ===
Successfully applied all changes from /path/to/63.0.0 to /path/to/64.0.0
=======================

Note: A complete log is saved to /path/to/64.0.0/import.log
```
