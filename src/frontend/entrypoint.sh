#!/bin/sh

# Read secret from mounted secrets file
if [ -f "/run/secrets/dropbox_client_id" ]; then
    export REACT_APP_DROPBOX_CLIENT_ID=$(cat "/run/secrets/dropbox_client_id" | tr -d '\n' | tr -d '\r')
    echo "Set REACT_APP_DROPBOX_CLIENT_ID from secret"
else
    echo "No secrets file found at /run/secrets/dropbox_client_id"
fi

# Create config file with proper JSON structure
echo "{
    \"BACKEND_URL\": \"$REACT_APP_BACKEND_URL\",
    \"USE_ELECTRON_IPC\": \"$REACT_APP_USE_ELECTRON_IPC\",
    \"DROPBOX_CLIENT_ID\": \"$REACT_APP_DROPBOX_CLIENT_ID\"
}" > /usr/share/nginx/html/config.json

echo "Created config.json with content:"
cat /usr/share/nginx/html/config.json

# Execute the original Docker command
exec "$@"