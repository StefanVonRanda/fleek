package main

import (
	"os"
	"os/exec"
	"strings"
)

type Protocol int

const (
	protoHalf Protocol = iota
	protoTGP
)

func detectProtocol() Protocol {
	override := strings.ToLower(os.Getenv("FLEEK_PROTOCOL"))
	switch override {
	case "tgp", "kitty":
		return protoTGP
	case "half", "halfblock", "halfblocks", "unicode":
		return protoHalf
	}
	switch strings.ToLower(appConfig.Images.Protocol) {
	case "tgp", "kitty":
		return protoTGP
	case "half", "halfblock", "halfblocks":
		return protoHalf
	}
	if !stdoutIsTerminal() {
		return protoHalf
	}
	if os.Getenv("TMUX") != "" {
		client := tmuxClientName()
		if strings.Contains(client, "ghostty") || strings.Contains(client, "kitty") || strings.Contains(client, "wezterm") {
			return protoTGP
		}
		return protoHalf
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "ghostty") || strings.Contains(term, "kitty") || strings.Contains(term, "wezterm") {
		return protoTGP
	}
	return protoHalf
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func tmuxClientName() string {
	output, err := exec.Command("tmux", "display-message", "-p", "#{client_termname}").Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(output)))
}

func protocolName(protocol Protocol) string {
	if protocol == protoTGP {
		return "kitty graphics (ghostty)"
	}
	return "half blocks"
}
