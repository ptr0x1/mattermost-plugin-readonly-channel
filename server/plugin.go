package main

import (
	"sync"

	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// Plugin : This plugin will make sure that only the configured bot
// can post in a channel - making it read-only.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// botID of the created bot account.
	botID string

	// backgroundJob is a job that executes periodically on only one plugin instance at a time
	backgroundJob *cluster.Job
}
