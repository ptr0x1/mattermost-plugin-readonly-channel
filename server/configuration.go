package main

import (
	"reflect"

	"encoding/json"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
type configuration struct {
	// The user to use as part of the read-only plugin, created automatically if it does not exist.
	Username string

	// The channel to use as part of the read-only plugin, created for each team automatically if it does not exist.
	ChannelName string

	// TextStyle controls the text style of the messages posted by the read-only user.
	TextStyle string

	// disabled tracks whether or not the plugin has been disabled after activation. It always starts enabled.
	disabled bool

	// readonlyUserID is the id of the user specified above.
	readonlyUserID string

	// readonlyChannelIDs maps team ids to the channels created for each using the channel name above.
	readonlyChannelIDs map[string]string
}

// Clone deep copies the configuration. Your implementation may only require a shallow copy if
// your configuration has no reference types.
func (c *configuration) Clone() *configuration {
	// Deep copy readonlyChannelIDs, a reference type.
	readonlyChannelIDs := make(map[string]string)
	for key, value := range c.readonlyChannelIDs {
		readonlyChannelIDs[key] = value
	}

	return &configuration{
		Username:           c.Username,
		ChannelName:        c.ChannelName,
		TextStyle:          c.TextStyle,
		disabled:           c.disabled,
		readonlyUserID:     c.readonlyUserID,
		readonlyChannelIDs: readonlyChannelIDs,
	}
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

func (p *Plugin) diffConfiguration(newConfiguration *configuration) {
	oldConfiguration := p.getConfiguration()
	configurationDiff := make(map[string]interface{})

	if newConfiguration.Username != oldConfiguration.Username {
		configurationDiff["username"] = newConfiguration.Username
	}
	if newConfiguration.ChannelName != oldConfiguration.ChannelName {
		configurationDiff["channel_name"] = newConfiguration.ChannelName
	}
	if newConfiguration.TextStyle != oldConfiguration.TextStyle {
		configurationDiff["text_style"] = newConfiguration.ChannelName
	}

	if len(configurationDiff) == 0 {
		return
	}

	teams, err := p.API.GetTeams()
	if err != nil {
		p.API.LogWarn("failed to query teams OnConfigChange", "err", err)
		return
	}

	for _, team := range teams {
		_, ok := newConfiguration.readonlyChannelIDs[team.Id]
		if !ok {
			p.API.LogWarn("No read-only channel id for team", "team", team.Id)
			continue
		}

		_, jsonErr := json.Marshal(newConfiguration)
		if jsonErr != nil {
			p.API.LogWarn("failed to marshal new configuration", "err", err)
			return
		}
	}
}

// OnConfigurationChange is invoked when configuration changes may have been made.
//
// This read-only implementation ensures the configured read-only user and channel are created for use
// by the plugin.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)
	var err error

	// Load the public configuration fields from the Mattermost server configuration.
	if loadConfigErr := p.API.LoadPluginConfiguration(configuration); loadConfigErr != nil {
		return errors.Wrap(loadConfigErr, "failed to load plugin configuration")
	}

	configuration.readonlyUserID, err = p.ensureROUser(configuration)
	if err != nil {
		return errors.Wrap(err, "failed to ensure read-only user")
	}

	botID, ensureBotError := p.Helpers.EnsureBot(&model.Bot{
		Username:    "readonlybot",
		DisplayName: "Read-only Bot",
		Description: "A bot account created by the read-only plugin.",
	}, plugin.IconImagePath("/assets/github.svg"))
	if ensureBotError != nil {
		return errors.Wrap(ensureBotError, "failed to ensure demo bot.")
	}

	p.botID = botID

	configuration.readonlyChannelIDs, err = p.ensureROChannels(configuration)
	if err != nil {
		return errors.Wrap(err, "failed to ensure demo channels")
	}

	p.diffConfiguration(configuration)

	p.setConfiguration(configuration)

	return nil
}

func (p *Plugin) ensureROUser(configuration *configuration) (string, error) {
	var err *model.AppError

	// Check for the configured user. Ignore any error, since it's hard to distinguish runtime
	// errors from a user simply not existing.
	user, _ := p.API.GetUserByUsername(configuration.Username)

	// Ensure the configured user exists.
	if user == nil {
		user, err = p.API.CreateUser(&model.User{
			Username:  configuration.Username,
			Password:  "ROUserPassword1",
			Email:     "rouser@zapatva.com",
			Nickname:  "ROUser",
			FirstName: "ROUser",
			Position:  "Bot",
		})

		if err != nil {
			return "", err
		}
	}

	teams, err := p.API.GetTeams()
	if err != nil {
		return "", err
	}

	for _, team := range teams {
		// Ignore any error.
		p.API.CreateTeamMember(team.Id, configuration.readonlyUserID)
	}

	return user.Id, nil
}

func (p *Plugin) ensureROChannels(configuration *configuration) (map[string]string, error) {
	teams, err := p.API.GetTeams()
	if err != nil {
		return nil, err
	}

	readonlyChannelIDs := make(map[string]string)
	for _, team := range teams {
		// Check for the configured channel. Ignore any error, since it's hard to
		// distinguish runtime errors from a channel simply not existing.
		channel, _ := p.API.GetChannelByNameForTeamName(team.Name, configuration.ChannelName, false)

		// Ensure the configured channel exists - if it does not we dont save it.
		if channel != nil {
			readonlyChannelIDs[team.Id] = channel.Id
		}
	}

	return readonlyChannelIDs, nil
}

// setEnabled wraps setConfiguration to configure if the plugin is enabled.
func (p *Plugin) setEnabled(enabled bool) {
	var configuration = p.getConfiguration().Clone()
	configuration.disabled = !enabled

	p.setConfiguration(configuration)
}
