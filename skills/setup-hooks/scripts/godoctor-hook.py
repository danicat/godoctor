#!/usr/bin/env python3
import sys
import json
import re

def allow():
    return {"decision": "allow", "reason": "GoDoctor hook: operation permitted"}

def deny(reason):
    return {"decision": "deny", "reason": reason}

def main():
	try:
		input_data = sys.stdin.read()
		if not input_data.strip():
			print(json.dumps(allow()))
			return

		payload = json.loads(input_data)
		tool_call = payload.get("toolCall") or payload.get("tool_call") or payload
		name = (tool_call.get("name") or tool_call.get("tool_name") or tool_call.get("tool") or
				payload.get("name") or payload.get("tool") or "")
		args = (tool_call.get("args") or tool_call.get("arguments") or tool_call.get("parameters") or
				payload.get("args") or payload.get("arguments") or payload.get("parameters") or {})

		if name == "run_command":
			cmd = args.get("CommandLine") or args.get("command_line") or args.get("command") or ""
			if "go build" in cmd:
				print(json.dumps(deny("BLOCKED by GoDoctor: Direct 'go build' execution via run_command is prohibited. Please use the 'smart_build' MCP tool instead.")))
				return
			if "go test" in cmd:
				print(json.dumps(deny("BLOCKED by GoDoctor: Direct 'go test' execution via run_command is prohibited. Please use the 'smart_build' MCP tool instead.")))
				return
			if "go mod init" in cmd:
				print(json.dumps(deny("BLOCKED by GoDoctor: Direct 'go mod init' execution is prohibited. Please use the 'add_dependencies' MCP tool instead.")))
				return
			if "go get" in cmd:
				print(json.dumps(deny("BLOCKED by GoDoctor: Direct 'go get' execution is prohibited. Please use the 'add_dependencies' MCP tool instead.")))
				return

			# Check shell file editing or reading on .go files
			if re.search(r'\b(cat|sed|awk|grep|tee|echo|head|tail|vi|nano)\b.*\.go\b', cmd) or re.search(r'<<.*\.go', cmd):
				print(json.dumps(deny("BLOCKED by GoDoctor: Operating on .go files via shell commands (sed/awk/cat/heredoc/etc.) is prohibited. Please use 'smart_edit', 'smart_multi_edit', or 'smart_read' MCP tools instead.")))
				return

		elif name == "view_file":
			path = args.get("AbsolutePath") or args.get("absolute_path") or args.get("path") or ""
			if path.lower().endswith(".go"):
				print(json.dumps(deny("BLOCKED by GoDoctor: Using 'view_file' on Go source files (.go) is prohibited. Please use the 'smart_read' MCP tool instead.")))
				return

		elif name in ("write_to_file", "replace_file_content", "multi_replace_file_content"):
			path1 = args.get("TargetFile") or args.get("target_file") or args.get("path") or ""
			path2 = args.get("AbsolutePath") or args.get("absolute_path") or ""
			if path1.lower().endswith(".go") or path2.lower().endswith(".go"):
				print(json.dumps(deny("BLOCKED by GoDoctor: Using low-level file editing tools on Go source files (.go) is prohibited. Please use 'smart_edit' or 'smart_multi_edit' MCP tools instead.")))
				return

		print(json.dumps(allow()))

	except Exception as e:
		print(json.dumps({"decision": "allow", "reason": f"GoDoctor hook error: {str(e)}"}))

if __name__ == "__main__":
    main()
