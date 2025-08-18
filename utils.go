package main

import (
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Clears filename-prohibited characters
func ClearFilename(name string) string {
	// Remove null bytes (dangerous)
	name = strings.ReplaceAll(name, "\x00", "")

	// Replace slashes (forbidden in Linux filenames)
	name = strings.ReplaceAll(name, "/", "_")

	// Optionally, remove other control characters
	reControl := regexp.MustCompile(`[[:cntrl:]]`)
	name = reControl.ReplaceAllString(name, "")

	name = strings.TrimSpace(name)

	// Enforce length limit (Linux NAME_MAX is usually 255 bytes)
	for len(name) > 255 {
		// Truncate safely without breaking Unicode
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}

	if name == "" {
		name = "unnamed"
	}

	return name
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start()
}

