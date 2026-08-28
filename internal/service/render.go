package service

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

func renderLaunchd(executable, stateDir string, owner StateOwner) []byte {
	var environment strings.Builder
	writePlistEnvironment(&environment, "LAN_NICK_STATE_DIR", stateDir)
	if owner.UID != "" {
		writePlistEnvironment(&environment, "LAN_NICK_STATE_UID", owner.UID)
		writePlistEnvironment(&environment, "LAN_NICK_STATE_GID", owner.GID)
	}
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>serve</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
%s  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ProcessType</key>
  <string>Background</string>
</dict>
</plist>
`, launchdLabel, html.EscapeString(executable), environment.String()))
}

func writePlistEnvironment(output *strings.Builder, key, value string) {
	fmt.Fprintf(output, "    <key>%s</key>\n    <string>%s</string>\n", html.EscapeString(key), html.EscapeString(value))
}

func renderSystemd(executable, stateDir string, owner StateOwner) []byte {
	var environment strings.Builder
	fmt.Fprintf(&environment, "Environment=%s\n", systemdQuote("LAN_NICK_STATE_DIR="+stateDir))
	if owner.UID != "" {
		fmt.Fprintf(&environment, "Environment=%s\n", systemdQuote("LAN_NICK_STATE_UID="+owner.UID))
		fmt.Fprintf(&environment, "Environment=%s\n", systemdQuote("LAN_NICK_STATE_GID="+owner.GID))
	}
	return []byte(fmt.Sprintf(`[Unit]
Description=%s
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=%s serve
%sRestart=on-failure
RestartSec=2s

[Install]
WantedBy=multi-user.target
`, DisplayName, systemdQuote(executable), environment.String()))
}

func systemdQuote(value string) string {
	return strconv.Quote(strings.ReplaceAll(value, "%", "%%"))
}
