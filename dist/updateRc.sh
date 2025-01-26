#!/bin/bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.9.1 or 1.9.1-alpha01 or 1.9.1-beta02 or 1.9.1-rc03"
    exit 1
fi

# Remove leading 'v' if present
VERSION=${1#v}

# Split version into parts
BASE_VERSION=$(echo $VERSION | cut -d'-' -f1)
PRE_RELEASE=$(echo $VERSION | cut -s -d'-' -f2)

# Convert base version (1.9.1) to comma format (1,9,1)
BASE_VERSION_COMMA=$(echo $BASE_VERSION | sed 's/\./,/g')

# Determine the fourth version number based on pre-release type
if [[ -z "$PRE_RELEASE" ]]; then
    # Release version
    FOURTH_NUM=400
elif [[ "$PRE_RELEASE" = alpha[0-9][0-9] ]]; then
    # Alpha version: 100 + number
    ALPHA_NUM=${PRE_RELEASE#alpha}
    FOURTH_NUM=$((100 + 10#$ALPHA_NUM))
elif [[ "$PRE_RELEASE" = beta[0-9][0-9] ]]; then
    # Beta version: 200 + number
    BETA_NUM=${PRE_RELEASE#beta}
    FOURTH_NUM=$((200 + 10#$BETA_NUM))
elif [[ "$PRE_RELEASE" = rc[0-9][0-9] ]]; then
    # RC version: 300 + number
    RC_NUM=${PRE_RELEASE#rc}
    FOURTH_NUM=$((300 + 10#$RC_NUM))
else
    echo "Invalid pre-release format. Use alphaNN, betaNN, or rcNN (where NN is two digits)"
    exit 1
fi

# Create final version string with comma
VERSION_COMMA="$BASE_VERSION_COMMA,$FOURTH_NUM"

# Create three-part version for ISS (e.g., 1.9.1210 from 1,9,1,210)
IFS='.' read -r MAJOR MINOR PATCH <<< "$BASE_VERSION"
ISS_VERSION="$MAJOR.$MINOR.$PATCH$FOURTH_NUM"

# Create resource.rc from template
sed -e "s/{{VERSION}}/$VERSION/g" \
    -e "s/{{VERSION_COMMA}}/$VERSION_COMMA/g" \
    dist/resource.rc.template > dist/resource.rc

# Update setup.iss version with three-part version number
sed -i "s/^AppVersion=.*/AppVersion=$ISS_VERSION/" dist/setup.iss
sed -i "s/^VersionInfoVersion=.*/VersionInfoVersion=$ISS_VERSION/" dist/setup.iss

# Update release.yaml with version information
sed -i "s/\(FileVersion = \).*/\1\"$MAJOR.$MINOR.$PATCH.$FOURTH_NUM\"/" .github/workflows/release.yaml
sed -i "s/\(ProductVersion = \).*/\1\"$MAJOR.$MINOR.$PATCH\"/" .github/workflows/release.yaml

# Update FyneApp.toml with the new version
sed -i "s/^Version = .*/Version = \"$MAJOR.$MINOR.$PATCH\"/" FyneApp.toml
sed -i "s/^Build = .*/Build = $PATCH$FOURTH_NUM/" FyneApp.toml

# Generate .syso file with rsrc
# rsrc -manifest dist/resource.rc -o owlcms.syso

echo "Updated release.yaml, setup.iss, and FyneApp.toml with version $VERSION (build number: $FOURTH_NUM, ISS version: $ISS_VERSION)"


