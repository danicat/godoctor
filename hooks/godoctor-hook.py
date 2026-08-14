#!/usr/bin/env python3
import sys
import json
import re

def allow(reason="GoDoctor hook: operation permitted"):
    return {"decision": "allow", "reason": reason}

def deny(reason):
    return {"decision": "deny", "reason": reason}

def is_allowed_go_test_command(cmd):
    # Allow advanced testing workflows: race detector, fuzzing, benchmarking
    if re.search(r'\bgo\s+test\b', cmd):
        if re.search(r'-(race|fuzz|bench)\b', cmd):
            return True
        return False
    return False

def main():
    try:
        input_data = sys.stdin.read()
        if not input_data.strip():
            print(json.dumps(allow()))
            sys.exit(0)

        payload = json.loads(input_data)
        tool_call = payload.get("toolCall") or payload.get("tool_call") or payload
        name = (tool_call.get("name") or tool_call.get("tool_name") or tool_call.get("tool") or
                payload.get("name") or payload.get("tool") or "")
        args = (tool_call.get("args") or tool_call.get("arguments") or tool_call.get("parameters") or
                payload.get("args") or payload.get("arguments") or payload.get("parameters") or {})

        if name == "run_command":
            cmd = args.get("CommandLine") or args.get("command_line") or args.get("command") or ""
            
            # Check for direct go build
            if re.search(r'\bgo\s+build\b', cmd):
                print(json.dumps(deny("BLOCKED by GoDoctor: Direct 'go build' execution via run_command is prohibited. Please use the 'smart_build' MCP tool instead.")))
                sys.exit(0)

            # Check for dependency management commands
            if re.search(r'\bgo\s+(get|mod\s+init|mod\s+tidy)\b', cmd):
                print(json.dumps(deny("BLOCKED by GoDoctor: Direct dependency/module modification commands are prohibited. Please use the 'add_dependencies' or 'smart_build' MCP tools instead.")))
                sys.exit(0)

            # Check for go test commands
            if re.search(r'\bgo\s+test\b', cmd):
                if not is_allowed_go_test_command(cmd):
                    print(json.dumps(deny("BLOCKED by GoDoctor: Standard 'go test' execution via run_command is prohibited. Please use 'smart_test' or 'smart_build' MCP tools instead. (Note: 'run_command' is allowed for -race, -fuzz, and -bench flags).")))
                    sys.exit(0)

            # Check shell file editing or reading on .go files
            if re.search(r'\b(cat|sed|awk|grep|tee|echo|head|tail|vi|nano)\b.*\.go\b', cmd) or re.search(r'<<.*\.go', cmd):
                print(json.dumps(deny("BLOCKED by GoDoctor: Operating on .go files via shell commands (sed/awk/cat/heredoc/etc.) is prohibited. Please use 'smart_edit', 'smart_multi_edit', or 'smart_read' MCP tools instead.")))
                sys.exit(0)

        elif name == "view_file":
            path = args.get("AbsolutePath") or args.get("absolute_path") or args.get("path") or ""
            if path.lower().endswith(".go"):
                print(json.dumps(deny("BLOCKED by GoDoctor: Using 'view_file' on Go source files (.go) is prohibited. Please use the 'smart_read' MCP tool instead.")))
                sys.exit(0)

        elif name in ("write_to_file", "replace_file_content", "multi_replace_file_content"):
            path1 = args.get("TargetFile") or args.get("target_file") or args.get("path") or ""
            path2 = args.get("AbsolutePath") or args.get("absolute_path") or ""
            if path1.lower().endswith(".go") or path2.lower().endswith(".go"):
                print(json.dumps(deny("BLOCKED by GoDoctor: Using low-level file editing tools on Go source files (.go) is prohibited. Please use 'smart_edit' or 'smart_multi_edit' MCP tools instead.")))
                sys.exit(0)

        print(json.dumps(allow()))
        sys.exit(0)

    except Exception as e:
        # Fallback to allow on unexpected hook exception with code 0 to prevent crashing runner
        print(json.dumps({"decision": "allow", "reason": f"GoDoctor hook error: {str(e)}"}))
        sys.exit(0)

if __name__ == "__main__":
    main()
