#!/usr/bin/env bash
set -e

GLOBAL=1
UNINSTALL=0

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --global) GLOBAL=1 ;;
    --workspace) GLOBAL=0 ;;
    --uninstall) UNINSTALL=1 ;;
    -h|--help)
      echo "Usage: $0 [--global | --workspace] [--uninstall]"
      exit 0
      ;;
    *) echo "Unknown parameter passed: $1"; exit 1 ;;
  esac
  shift
done

# Detect config directory
if [ -d "$HOME/.gemini/antigravity-cli" ]; then
  CONFIG_DIR="$HOME/.gemini/antigravity-cli"
elif [ -d "$HOME/.gemini/config" ]; then
  CONFIG_DIR="$HOME/.gemini/config"
else
  CONFIG_DIR="$HOME/.gemini/config"
fi

if [ "$GLOBAL" -eq 1 ]; then
  HOOKS_JSON="$CONFIG_DIR/hooks.json"
  HOOKS_DIR="$CONFIG_DIR/hooks"
else
  HOOKS_JSON=".agents/hooks.json"
  HOOKS_DIR=".agents/hooks"
fi

HOOK_SCRIPT="$HOOKS_DIR/godoctor-hook.py"

if [ "$UNINSTALL" -eq 1 ]; then
  echo "Uninstalling GoDoctor hooks..."
  if [ -f "$HOOK_SCRIPT" ]; then
    rm -f "$HOOK_SCRIPT"
  fi
  if [ -f "$HOOKS_JSON" ]; then
    # Remove godoctor entries
    python3 -c '
import json, sys
with open(sys.argv[1], "r") as f:
    try:
        data = json.load(f)
    except:
        sys.exit(0)

if "godoctor-hooks" in data:
    del data["godoctor-hooks"]

if "PreToolUse" in data:
    data["PreToolUse"] = [h for h in data["PreToolUse"] if h.get("name") != "godoctor-hook"]
    if not data["PreToolUse"]:
        del data["PreToolUse"]

with open(sys.argv[1], "w") as f:
    json.dump(data, f, indent=2)
' "$HOOKS_JSON"
  fi
  echo "Done."
  exit 0
fi


echo "Installing GoDoctor hooks to $HOOKS_JSON..."

mkdir -p "$HOOKS_DIR"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cp "$SCRIPT_DIR/godoctor-hook.py" "$HOOK_SCRIPT"
chmod +x "$HOOK_SCRIPT"

# Ensure hooks.json exists
if [ ! -f "$HOOKS_JSON" ]; then
  mkdir -p "$(dirname "$HOOKS_JSON")"
  echo "{}" > "$HOOKS_JSON"
fi

# Add hook to hooks.json
python3 -c '
import json, sys
with open(sys.argv[1], "r") as f:
    try:
        data = json.load(f)
    except:
        data = {}

if "godoctor-hooks" not in data:
    data["godoctor-hooks"] = {
        "enabled": True,
        "PreToolUse": []
    }

suite = data["godoctor-hooks"]
if "PreToolUse" not in suite:
    suite["PreToolUse"] = []

# Remove existing matcher entry if present
suite["PreToolUse"] = [
    item for item in suite["PreToolUse"] 
    if not any(h.get("command") == sys.argv[2] for h in item.get("hooks", []))
]

hook_entry = {
    "matcher": "run_command|view_file|write_to_file|replace_file_content|multi_replace_file_content",
    "hooks": [
        {
            "type": "command",
            "command": sys.argv[2],
            "timeout": 15
        }
    ]
}
suite["PreToolUse"].append(hook_entry)

with open(sys.argv[1], "w") as f:
    json.dump(data, f, indent=2)
' "$HOOKS_JSON" "$HOOK_SCRIPT"

echo "Setup complete."

