//go:build !darwin

package seatbelt

import "errors"

// Enter fails: Seatbelt exists on macOS only, and crannog does not run
// without it.
func Enter() error {
	return errors.New("crannog runs on macOS only: it needs Seatbelt (sandbox-exec)")
}
