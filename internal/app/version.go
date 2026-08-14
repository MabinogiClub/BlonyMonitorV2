package app

import "blonymonitorv2/internal/config"

// GetClientVersion returns the version injected by the release build.
func (a *App) GetClientVersion() string {
	return config.ClientVersion
}
