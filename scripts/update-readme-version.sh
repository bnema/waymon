#!/bin/bash

# Script to update version links in README.md
# Usage: ./scripts/update-readme-version.sh <version>

set -e

VERSION="$1"
README_FILE="README.md"

if [[ -z "$VERSION" ]]; then
    echo "Error: Version parameter is required"
    echo "Usage: $0 <version>"
    exit 1
fi

if [[ ! -f "$README_FILE" ]]; then
    echo "Error: README.md not found"
    exit 1
fi

echo "Updating README.md version links to $VERSION..."

# Create a backup
cp "$README_FILE" "$README_FILE.bak"

# Update Arch Linux package link
sed -i "s|https://github.com/bnema/waymon/releases/download/v[^/]*/waymon_[^_]*_linux_amd64\.pkg\.tar\.zst|https://github.com/bnema/waymon/releases/download/$VERSION/waymon_${VERSION#v}_linux_amd64.pkg.tar.zst|g" "$README_FILE"

# Update Arch Linux install command
sed -i "s|sudo pacman -U waymon_[^_]*_linux_amd64\.pkg\.tar\.zst|sudo pacman -U waymon_${VERSION#v}_linux_amd64.pkg.tar.zst|g" "$README_FILE"

# Update Debian package link
sed -i "s|https://github.com/bnema/waymon/releases/download/v[^/]*/waymon_[^_]*_linux_amd64\.deb|https://github.com/bnema/waymon/releases/download/$VERSION/waymon_${VERSION#v}_linux_amd64.deb|g" "$README_FILE"

# Update Debian install command
sed -i "s|sudo dpkg -i waymon_[^_]*_linux_amd64\.deb|sudo dpkg -i waymon_${VERSION#v}_linux_amd64.deb|g" "$README_FILE"

# Update RPM package link
sed -i "s|https://github.com/bnema/waymon/releases/download/v[^/]*/waymon_[^_]*_linux_amd64\.rpm|https://github.com/bnema/waymon/releases/download/$VERSION/waymon_${VERSION#v}_linux_amd64.rpm|g" "$README_FILE"

# Update RPM install command
sed -i "s|sudo rpm -i waymon_[^_]*_linux_amd64\.rpm|sudo rpm -i waymon_${VERSION#v}_linux_amd64.rpm|g" "$README_FILE"

# Update tarball link
sed -i "s|https://github.com/bnema/waymon/releases/download/v[^/]*/waymon_Linux_x86_64\.tar\.gz|https://github.com/bnema/waymon/releases/download/$VERSION/waymon_Linux_x86_64.tar.gz|g" "$README_FILE"

# Verify changes were made
if ! diff -q "$README_FILE" "$README_FILE.bak" > /dev/null; then
    echo "✅ Successfully updated README.md version links to $VERSION"
    rm "$README_FILE.bak"
else
    echo "⚠️  No changes made to README.md"
    rm "$README_FILE.bak"
fi

echo "Done!"