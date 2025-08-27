#!/bin/bash

# Auto Release script for hymx
# This script reads the Variant from schema.go and creates a release automatically
# Usage: ./scripts/auto-release.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are installed
check_dependencies() {
    print_info "Checking dependencies..."
    
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed. Please install it first:"
        echo "  brew install gh"
        echo "  or visit: https://cli.github.com/"
        exit 1
    fi
    
    if ! command -v make &> /dev/null; then
        print_error "make is not installed."
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        print_error "jq is not installed. Please install it first:"
        echo "  brew install jq"
        echo "  or visit: https://stedolan.github.io/jq/"
        exit 1
    fi
    
    # Check if user is authenticated with GitHub CLI
    if ! gh auth status &> /dev/null; then
        print_error "You are not authenticated with GitHub CLI. Please run:"
        echo "  gh auth login"
        exit 1
    fi
    
    print_success "All dependencies are available."
}

# Check if current user has admin permissions for the repository
check_admin_permissions() {
    print_info "Checking repository permissions..."
    
    # Get current user
    CURRENT_USER=$(gh api user --jq '.login' 2>/dev/null)
    if [ -z "$CURRENT_USER" ]; then
        print_error "Failed to get current user information."
        exit 1
    fi
    
    # Get repository information
    REPO_INFO=$(gh repo view --json owner,name 2>/dev/null)
    if [ -z "$REPO_INFO" ]; then
        print_error "Failed to get repository information. Make sure you're in a git repository with GitHub remote."
        exit 1
    fi
    
    REPO_OWNER=$(echo "$REPO_INFO" | jq -r '.owner.login')
    REPO_NAME=$(echo "$REPO_INFO" | jq -r '.name')
    
    print_info "Repository: $REPO_OWNER/$REPO_NAME"
    print_info "Current user: $CURRENT_USER"
    
    # Check if user is the repository owner
    if [ "$CURRENT_USER" = "$REPO_OWNER" ]; then
        print_success "User is repository owner - permission granted."
        return 0
    fi
    
    # Check if user has admin permissions via collaborator API
    USER_PERMISSION=$(gh api "repos/$REPO_OWNER/$REPO_NAME/collaborators/$CURRENT_USER/permission" --jq '.permission' 2>/dev/null || echo "none")
    
    if [ "$USER_PERMISSION" = "admin" ]; then
        print_success "User has admin permissions - permission granted."
        return 0
    fi
    
    # Check if user is member of organization with admin role (for org repos)
    if [ "$REPO_OWNER" != "$CURRENT_USER" ]; then
        ORG_MEMBERSHIP=$(gh api "orgs/$REPO_OWNER/members/$CURRENT_USER" --jq '.state' 2>/dev/null || echo "none")
        if [ "$ORG_MEMBERSHIP" = "active" ]; then
            # Check if user has admin role in organization
            ORG_ROLE=$(gh api "orgs/$REPO_OWNER/memberships/$CURRENT_USER" --jq '.role' 2>/dev/null || echo "none")
            if [ "$ORG_ROLE" = "admin" ]; then
                print_success "User has organization admin role - permission granted."
                return 0
            fi
        fi
    fi
    
    # Permission denied
    print_error "Permission denied: You don't have admin access to this repository."
    print_error "Only repository administrators can create releases."
    print_info "Current permission level: $USER_PERMISSION"
    print_info "Required permission level: admin"
    exit 1
}

# Read version from node/schema/default.go
read_version() {
    if [ ! -f "node/schema/default.go" ]; then
        print_error "node/schema/default.go not found!"
        exit 1
    fi
    
    VERSION=$(grep 'NodeVersion.*=' node/schema/default.go | sed 's/.*"\(.*\)".*/\1/')
    
    if [ -z "$VERSION" ]; then
        print_error "Could not extract version from node/schema/default.go"
        exit 1
    fi
    
    # Validate version format
    if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        print_error "Invalid version format in node/schema/default.go: $VERSION"
        print_error "Expected format: vX.Y.Z (e.g., v0.1.3)"
        exit 1
    fi
    
    print_info "Found version in node/schema/default.go: $VERSION"
}

# Check if we're in the right directory
check_project_root() {
    if [ ! -f "go.mod" ] || [ ! -f "Makefile" ] || [ ! -f "node/schema/default.go" ]; then
        print_error "This script must be run from the project root directory."
        exit 1
    fi
}

# Check if working directory is clean
check_git_status() {
    if [ -n "$(git status --porcelain)" ]; then
        print_error "Working directory is not clean. Please commit or stash your changes."
        git status --short
        exit 1
    fi
}

# Check if tag already exists
check_existing_tag() {
    if git tag -l | grep -q "^$VERSION$"; then
        print_warning "Tag $VERSION already exists!"
        read -p "Do you want to delete the existing tag and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deleting existing tag: $VERSION"
            git tag -d "$VERSION" || true
            git push origin --delete "$VERSION" || true
        else
            print_info "Release cancelled."
            exit 0
        fi
    fi
}

# Create git tag
create_tag() {
    print_info "Creating tag: $VERSION"
    git tag -a "$VERSION" -m "Release $VERSION"
    
    print_info "Pushing tag to origin..."
    git push origin "$VERSION"
    
    print_success "Tag $VERSION created and pushed successfully!"
}

# Build all platform binaries
build_binaries() {
    print_info "Building binaries for all platforms..."
    
    # Clean build directory
    rm -rf ./build/
    
    # Build all platforms
    make build-all
    
    # Check if build was successful
    if [ ! -d "./build" ] || [ -z "$(ls -A ./build)" ]; then
        print_error "Build failed or no binaries were created!"
        exit 1
    fi
    
    print_success "All binaries built successfully!"
    ls -la ./build/
}

# Read release notes from local file
read_release_notes() {
    local release_notes_file="release_notes_${VERSION}.md"
    
    if [ ! -f "$release_notes_file" ]; then
        print_error "Release notes file not found: $release_notes_file"
        print_error "Please create the release notes file before running the release."
        exit 1
    fi
    
    print_info "Reading release notes from: $release_notes_file"
    RELEASE_NOTES=$(cat "$release_notes_file")
    
    if [ -z "$RELEASE_NOTES" ]; then
        print_error "Release notes file is empty: $release_notes_file"
        print_error "Please add content to the release notes file."
        exit 1
    fi
}


# Create GitHub release and upload binaries
create_github_release() {
    print_info "Creating GitHub release: $VERSION"
    
    # Read release notes from file or use default
    read_release_notes
    
    # Create release with GitHub CLI
    gh release create "$VERSION" \
        --title "$VERSION" \
        --notes "$RELEASE_NOTES" \
        ./build/*
    
    print_success "GitHub release $VERSION created successfully!"
    
    # Get the release URL
    REPO_URL=$(git config --get remote.origin.url | sed 's/.*github.com[:\/]\([^\/]*\/[^\/]*\).*/\1/' | sed 's/\.git$//')
    print_info "Release URL: https://github.com/$REPO_URL/releases/tag/$VERSION"
}

# Main function
main() {
    print_info "Starting auto-release process..."
    
    check_project_root
    check_dependencies
    check_admin_permissions
    read_version
    
    print_info "Preparing to release version: $VERSION"
    
    # Check git status
    check_git_status
    
    # Pull latest changes
    print_info "Pulling latest changes..."
    git pull origin main
    
    # Check existing tag
    check_existing_tag
    
    # Confirm release
    echo
    print_warning "This will:"
    echo "  1. Create and push git tag: $VERSION"
    echo "  2. Build binaries for all platforms"
    echo "  3. Create GitHub release with binaries"
    echo
    read -p "Do you want to proceed? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Release cancelled."
        exit 0
    fi
    
    # Execute release steps
    create_tag
    build_binaries
    create_github_release
    
    print_success "Auto-release completed successfully!"
    print_info "Version $VERSION has been released with all platform binaries."
}

# Run main function
main "$@"