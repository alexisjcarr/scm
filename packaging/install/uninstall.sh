#!/usr/bin/env bash
set -euo pipefail

systemctl disable --now scmctld scmctld-agent 2>/dev/null || true
rm -f /usr/local/bin/scmctl /usr/local/bin/scmctld /usr/local/bin/scmctld-agent /usr/local/bin/scm-host-a-demo
rm -f /usr/local/libexec/scm-agent-fileop
rm -f /etc/systemd/system/scmctld.service /etc/systemd/system/scmctld-agent.service
rm -f /etc/sudoers.d/scmctld-agent
rm -rf /usr/local/share/doc/scm
rm -rf /usr/local/share/scm
systemctl daemon-reload
echo "removed installed binaries and unit files"
