#!/usr/bin/env bash
set -euo pipefail

systemctl disable --now scmctld scmctld-agent 2>/dev/null || true
rm -f /usr/local/bin/scmctl /usr/local/bin/scmctld /usr/local/bin/scmctld-agent
rm -f /etc/systemd/system/scmctld.service /etc/systemd/system/scmctld-agent.service
rm -rf /usr/local/share/doc/scm
systemctl daemon-reload
echo "removed installed binaries and unit files"
