#!/usr/bin/env python3
"""PreToolUse hook that denies a few high-risk shell command patterns.

Claude Code passes the hook input as JSON on stdin. This is a convenience
guard, not a security boundary: it does not see through every form of shell
indirection, so keep permission prompts on.
"""
import json
import re
import sys

try:
    payload = json.load(sys.stdin)
except Exception:
    sys.exit(0)

tool_input = payload.get("tool_input", {})
if not isinstance(tool_input, dict):
    sys.exit(0)
command = tool_input.get("command", "")
if not isinstance(command, str):
    sys.exit(0)

bad = (
    (r"(?:^|[;&|\s])git\s+push\s+[^\n]*--force(?:-with-lease)?(?:\s|$)", "Force-push requires human review"),
    (r"(?:^|[;&|\s])git\s+reset\s+--hard(?:\s|$)", "Hard reset would discard work"),
    (r"(?:^|[;&|\s])git\s+clean\s+-[^\n]*f", "git clean force is unsafe here"),
    (r"(?:^|[;&|\s])rm\s+-[A-Za-z]*r[A-Za-z]*f(?:\s|$)", "Recursive forced removal requires human review"),
)
for pattern, reason in bad:
    if re.search(pattern, command, flags=re.IGNORECASE):
        print(json.dumps({"hookSpecificOutput": {"hookEventName":"PreToolUse", "permissionDecision":"deny", "permissionDecisionReason":reason}}))
        sys.exit(0)

sys.exit(0)
