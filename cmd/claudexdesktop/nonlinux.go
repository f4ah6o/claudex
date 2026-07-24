//go:build !linux

package main

import "errors"

func runLinux() error {
	return errors.New("Linux ClaudexDesktop support is unavailable on this platform")
}

func showMessageLinux(string, string, bool) error {
	return errors.New("Linux message dialogs are unavailable on this platform")
}
