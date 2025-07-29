#!/bin/bash

set -euo pipefail

echo "Starting godoctor server and sending full MCP sequence..."

# Construct the full sequence of JSON-RPC messages
# Initialize, Initialized, tools/list, go-doc call, tools/list, help call
FULL_MCP_SEQUENCE=$(cat <<EOF
{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"capabilities": {}, "clientInfo": {"name": "Gemini CLI", "version": "1.0"}}}
{"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}}
{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "go-doc", "arguments": {"package_path": "fmt"}}}
EOF
)

# Pipe the full sequence to bin/godoctor and capture all output
ALL_RESPONSE=$(echo -e "$FULL_MCP_SEQUENCE" | bin/godoctor 2>&1)

echo "Full Server Response:"
echo "$ALL_RESPONSE"
