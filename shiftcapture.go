package main

import (
	"os"
	"strings"
)

func shiftCaptureSequence(enabled bool) string {
	sequence := "\x1b[>0s"
	if enabled {
		sequence = "\x1b[>1s"
	}
	if os.Getenv("TMUX") != "" {
		escaped := strings.ReplaceAll(sequence, "\x1b", "\x1b\x1b")
		return "\x1bPtmux;" + escaped + "\x1b\\"
	}
	return sequence
}

func setShiftCapture(enabled bool) {
	_, _ = os.Stdout.WriteString(shiftCaptureSequence(enabled))
	_ = os.Stdout.Sync()
}
