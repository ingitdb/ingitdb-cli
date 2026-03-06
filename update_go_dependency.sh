#!/bin/bash
set -e
if [ -z "$1" ]; then
  echo "Usage: $0 <go_module_path>"
  exit 1
fi
MODULE_PATH=$1
echo "Updating $MODULE_PATH to the latest major version..."
# 1. Find the current major version from go.mod
ALL_VERSIONS=$(go list -m -f '{{.Path}}' all | grep "$MODULE_PATH/v[0-9]*" || true)
HIGHEST_MAJOR_VERSION=0
for path in $ALL_VERSIONS; do
  VERSION_NUMBER=$(echo "$path" | sed -E 's|.*v([0-9]+)|\1|')
  if [ "$VERSION_NUMBER" -gt "$HIGHEST_MAJOR_VERSION" ]; then
    HIGHEST_MAJOR_VERSION=$VERSION_NUMBER
  fi
done
# 2. Find the latest major version
LATEST_VERSION_PATH="$MODULE_PATH/v$HIGHEST_MAJOR_VERSION"
for i in $(seq $(($HIGHEST_MAJOR_VERSION + 1)) 100); do
  NEXT_VERSION_PATH="$MODULE_PATH/v$i"
  if go list -m "$NEXT_VERSION_PATH@latest" > /dev/null 2>&1; then
    LATEST_VERSION_PATH=$NEXT_VERSION_PATH
  else
    break
  fi
done
# 3. Replace all occurrences in the code
for path in $ALL_VERSIONS; do
  if [ "$path" != "$LATEST_VERSION_PATH" ]; then
    echo "Replacing all occurrences of '$path' with '$LATEST_VERSION_PATH'..."
    grep -rl "$path" . | env LC_ALL=C xargs sed -i '' "s|$path|$LATEST_VERSION_PATH|g"
  fi
done
echo "Running go mod tidy..."
go mod tidy
echo "Done."
