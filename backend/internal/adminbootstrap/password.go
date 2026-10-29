package adminbootstrap

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ReadPasswordInteractively prompts twice for a password without echoing.
// Prefers AIVION_ADMIN_PASSWORD_FILE when set (file contains only the password).
func ReadPasswordInteractively() (string, error) {
	if path := strings.TrimSpace(os.Getenv("AIVION_ADMIN_PASSWORD_FILE")); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("cannot read AIVION_ADMIN_PASSWORD_FILE")
		}
		password := strings.TrimRight(string(raw), "\r\n")
		if password == "" {
			return "", fmt.Errorf("AIVION_ADMIN_PASSWORD_FILE is empty")
		}
		return password, nil
	}

	fmt.Fprint(os.Stderr, "Admin password: ")
	passBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		// Fallback for non-TTY environments that still pipe a password via stdin once.
		reader := bufio.NewReader(os.Stdin)
		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(strings.TrimSpace(line)) == 0 {
			return "", fmt.Errorf("failed to read password")
		}
		passBytes = []byte(strings.TrimRight(line, "\r\n"))
	}
	password := string(passBytes)
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}

	fmt.Fprint(os.Stderr, "Confirm password: ")
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(strings.TrimSpace(line)) == 0 {
			return "", fmt.Errorf("failed to read password confirmation")
		}
		confirmBytes = []byte(strings.TrimRight(line, "\r\n"))
	}
	if password != string(confirmBytes) {
		return "", fmt.Errorf("passwords do not match")
	}
	return password, nil
}
