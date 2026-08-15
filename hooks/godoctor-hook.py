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

        # Block direct modification of Go files via built-in edit tools
        if name in ["write_to_file", "replace_file_content", "multi_replace_file_content", "sed_file"]:
            target = args.get("TargetFile") or args.get("target_file") or args.get("path") or args.get("target") or ""
            if target.endswith(".go"):
                print(json.dumps(deny(
                    f"GoDoctor guidance: Direct file modification of Go files via '{name}' is restricted. "
                    "Use GoDoctor MCP tools 'smart_edit' (for single-file compiler-verified changes) "
                    "or 'smart_multi_edit' (for atomic multi-file changes)."
                )))
                sys.exit(0)

        # Block reading Go files with standard view_file
        if name == "view_file":
            target = args.get("AbsolutePath") or args.get("absolute_path") or args.get("path") or ""
            if target.endswith(".go"):
                print(json.dumps(deny(
                    "GoDoctor guidance: Viewing Go files via 'view_file' is restricted. "
                    "Use GoDoctor MCP tool 'smart_read' to inspect Go files with AST parsing, "
                    "type-tag enrichment, and line ranges."
                )))
                sys.exit(0)

        # Intercept shell commands
        if name in ["run_command", "execute_command", "bash", "sh"]:
            cmd = args.get("CommandLine") or args.get("command") or args.get("cmd") or ""

            # Check for direct execution of godoctor or go doctor as a CLI command
            if re.search(r'(^|[;&|`]\s*|\$\(\s*)(go\s+doctor|godoctor)(\s+|$)', cmd):
                print(json.dumps(deny(
                    "godoctor is not a command to be called, it is an mcp server that exposes tools. "
                    "Use GoDoctor MCP tools directly: smart_read, smart_edit, smart_multi_edit, "
                    "smart_build, smart_test, test_query, mutation_test, list_files, add_dependencies, read_docs."
                )))
                sys.exit(0)

            # Check for shell redirection/modification of .go files
            if re.search(r'(>|>>|tee|sed|cat|echo)\s+.*\.go\b', cmd):
                print(json.dumps(deny(
                    "GoDoctor guidance: Modifying Go files via shell redirection or scripts is restricted. "
                    "Use 'smart_edit' or 'smart_multi_edit' to ensure syntax and type safety."
                )))
                sys.exit(0)

            # Check for raw go build / go vet
            if re.search(r'\bgo\s+(build|vet|fmt|modernize)\b', cmd):
                print(json.dumps(deny(
                    "GoDoctor guidance: Direct execution of 'go build/vet/fmt' is restricted. "
                    "Use GoDoctor MCP tool 'smart_build' to run the automated hygiene and build pipeline."
                )))
                sys.exit(0)

            # Check for raw go get / go mod
            if re.search(r'\bgo\s+(get|mod)\b', cmd):
                print(json.dumps(deny(
                    "GoDoctor guidance: Direct execution of 'go get' or 'go mod' is restricted. "
                    "Use GoDoctor MCP tool 'add_dependencies' to add packages and inspect documentation."
                )))
                sys.exit(0)

            # Check for raw go test
            if re.search(r'\bgo\s+test\b', cmd):
                if not is_allowed_go_test_command(cmd):
                    print(json.dumps(deny(
                        "GoDoctor guidance: Standard 'go test' is restricted. "
                        "Use GoDoctor MCP tool 'smart_test' (with level='fast'|'basic'|'benchmark'|'complete') "
                        "or 'mutation_test' for Selene test quality analysis. "
                        "Direct 'go test' is only permitted with -race, -fuzz, or -bench flags."
                    )))
                    sys.exit(0)

        print(json.dumps(allow()))
        sys.exit(0)

    except Exception as e:
        print(json.dumps(allow(f"GoDoctor hook error: {str(e)}")), file=sys.stderr)
        sys.exit(0)

if __name__ == "__main__":
    main()
