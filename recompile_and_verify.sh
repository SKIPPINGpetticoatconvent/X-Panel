#!/bin/bash
cd /home/ub/X-Panel
export TERM=xterm-256color
rm -f .agent/skills/coding-standards/bin/verify
echo "Compiling verify tool..." >verify_debug.log
if rustc .agent/skills/coding-standards/scripts/verify.rs -o .agent/skills/coding-standards/bin/verify >>verify_debug.log 2>&1; then
  echo "Compilation success. Running verify..." >>verify_debug.log
  chmod +x .agent/skills/coding-standards/bin/verify
  .agent/skills/coding-standards/bin/verify >>verify_debug.log 2>&1
else
  echo "Compilation failed!" >>verify_debug.log
fi
