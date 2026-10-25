//go:build tray && (linux || (darwin && cgo)) && !darwin

package cmd

const hasPrefsWindow = false

func (a *trayApp) openPreferences() {}
