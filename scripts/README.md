# Release Scripts

This directory contains scripts for automating the release process.

## auto-release.sh

Automatic release script that creates GitHub Releases based on the `Variant` variable in `schema/schema.go`.

### Features

1. **Automatic Version Detection**: Reads the `Variant` variable from `schema/schema.go` as the version number
2. **Git Tag Management**: Automatically creates and pushes Git tags
3. **Multi-platform Build**: Uses `make build-all` to build binaries for all platforms
4. **GitHub Release**: Uses GitHub CLI to create releases and upload binary files

### Prerequisites

1. **Release Notes File**: Create a release notes file before running the script
   ```bash
   # File naming format: release_notes_[VERSION].md
   # Example: release_notes_v0.1.3.md
   ```
   - The file must exist and contain content
   - Use the provided `notes_template.md` as a reference
   - The script will fail if the file is missing or empty

2. **GitHub CLI**: Install and authenticate GitHub CLI
   ```bash
   # macOS
   brew install gh
   
   # Authentication
   gh auth login
   ```

3. **jq**: JSON processing tool for parsing GitHub API responses
   ```bash
   # macOS
   brew install jq
   ```

4. **Make**: Ensure the make tool is installed on your system

5. **Administrator Permissions**: Only repository administrators can execute release operations
   - Repository owner
   - Collaborators with admin permissions
   - Organization administrators (for organization repositories)

6. **Git Permissions**: Ensure you have permissions to push tags and create releases

### Usage

1. **Create Release Notes**: First, create a release notes file for your version
   ```bash
   # Copy the template
   cp ./scripts/notes_template.md release_notes_v0.1.3.md
   
   # Edit the file with your release information
   # Replace [VERSION] with actual version number
   # Fill in the features, bug fixes, and other changes
   ```

2. **Run the Release Script**:
   ```bash
   # Execute from project root directory
   ./scripts/auto-release.sh
   ```

   The script will:
   - Automatically detect the version from `schema/schema.go`
   - Look for the corresponding `release_notes_v[VERSION].md` file
   - Fail if the release notes file is missing or empty

### Workflow

1. **Environment Check**: Verify dependency tools (GitHub CLI, jq, make) and Git authentication status
2. **Permission Verification**: Check if the current user has repository administrator permissions
3. **Version Reading**: Extract the `Variant` value from `schema/schema.go`
4. **Version Validation**: Ensure the version format follows `vX.Y.Z` specification
5. **Git Status Check**: Ensure the working directory is clean
6. **Tag Handling**: Check and handle existing tags
7. **User Confirmation**: Display operations to be performed and wait for confirmation
8. **Create Tag**: Create and push Git tag
9. **Build Binaries**: Execute `make build-all` to build all platforms
10. **Publish Release**: Create GitHub Release and upload binary files

### Example Output

```
[INFO] Starting auto-release process...
[INFO] Checking dependencies...
[SUCCESS] All dependencies are available.
[INFO] Checking repository permissions...
[INFO] Repository: your-org/hymx
[INFO] Current user: your-username
[SUCCESS] User is repository owner - permission granted.
[INFO] Found version in schema.go: v0.1.2
[INFO] Preparing to release version: v0.1.2
[INFO] Pulling latest changes...
[WARNING] This will:
  1. Create and push git tag: v0.1.2
  2. Build binaries for all platforms
  3. Create GitHub release with binaries

Do you want to proceed? (y/N): y
[INFO] Creating tag: v0.1.2
[SUCCESS] Tag v0.1.2 created and pushed successfully!
[INFO] Building binaries for all platforms...
[SUCCESS] All binaries built successfully!
[INFO] Creating GitHub release: v0.1.2
[SUCCESS] GitHub release v0.1.2 created successfully!
[INFO] Release URL: https://github.com/your-org/hymx/releases/tag/v0.1.2
[SUCCESS] Auto-release completed successfully!
```

### Error Handling

The script includes comprehensive error handling mechanisms:

- **Dependency Check**: Verify GitHub CLI, jq, and make tools
- **Authentication Verification**: Ensure GitHub CLI is authenticated
- **Permission Verification**: Ensure user has repository administrator permissions
- **Version Format**: Validate version number format
- **Git Status**: Ensure working directory is clean
- **Build Verification**: Ensure binary files are built successfully

### Notes

1. **Release Notes Requirement**: You MUST create a `release_notes_v[VERSION].md` file before running the script
   - The file must exist and contain content
   - Use `notes_template.md` as a starting point
   - The script will exit with an error if the file is missing or empty
2. **Version Format**: The `Variant` in `schema/schema.go` must be in `vX.Y.Z` format
3. **Working Directory**: The script must be executed from the project root directory
4. **Git Status**: Ensure all changes are committed before execution
5. **Network Connection**: Requires stable network connection to push tags and upload files
6. **Permission Requirements**: Requires repository write permissions and permission to create releases
7. **File Management**: Release notes files (`release_notes_*.md`) are automatically ignored by Git

## release.sh

Manual release script that allows specifying a version number for release (via GitHub Actions).

### Usage

```bash
./scripts/release.sh v0.1.3
```

This script will create the specified Git tag, triggering the GitHub Actions automatic build and release process.