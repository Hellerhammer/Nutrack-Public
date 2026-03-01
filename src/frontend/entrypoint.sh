#!/bin/sh

# Read secret from mounted secrets file
if [ -f "/run/secrets/dropbox_client_id" ]; then
    export REACT_APP_DROPBOX_CLIENT_ID=$(cat "/run/secrets/dropbox_client_id" | tr -d '\n' | tr -d '\r')
    echo "Set REACT_APP_DROPBOX_CLIENT_ID from secret"
else
    echo "No secrets file found at /run/secrets/dropbox_client_id"
fi

# Read existing DROPBOX_CLIENT_ID from baked-in config.json as fallback
BAKED_DROPBOX_CLIENT_ID=$(grep -o '"DROPBOX_CLIENT_ID": *"[^"]*"' /usr/share/nginx/html/config.json | grep -o '"[^"]*"$' | tr -d '"')

# Use env var if set, otherwise fall back to baked-in value
RESOLVED_DROPBOX_CLIENT_ID="${REACT_APP_DROPBOX_CLIENT_ID:-$BAKED_DROPBOX_CLIENT_ID}"

# Create config file with proper JSON structure
echo "{
    \"BACKEND_URL\": \"$REACT_APP_BACKEND_URL\",
    \"USE_ELECTRON_IPC\": \"$REACT_APP_USE_ELECTRON_IPC\",
    \"DROPBOX_CLIENT_ID\": \"$RESOLVED_DROPBOX_CLIENT_ID\"
}" > /usr/share/nginx/html/config.json

echo "Created config.json with content:"
cat /usr/share/nginx/html/config.json

# Execute the original Docker command
exec "$@"