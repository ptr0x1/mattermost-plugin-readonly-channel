package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// MessageWillBePosted is invoked when a message is posted by a user before it is committed to the
// database. If you also want to act on edited posts, see MessageWillBeUpdated. Return values
// should be the modified post or nil if rejected and an explanation for the user.
//
// If you don't need to modify or reject posts, use MessageHasBeenPosted instead.
//
// Note that this method will be called for posts created by plugins, including the plugin that created the post.
//
// This read-only implementation rejects posts in the read-only channel.
func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()

	if configuration.disabled {
		return post, ""
	}

	// Always allow posts by the read-only plugin user and read-only plugin bot.
	if post.UserId == p.botID || post.UserId == configuration.readonlyUserID {
		return post, ""
	}

	// Reject posts by other users in the read-only channels, effectively making it read-only.
	for _, channelID := range configuration.readonlyChannelIDs {
		if channelID == post.ChannelId {
			p.API.SendEphemeralPost(post.UserId, &model.Post{
				UserId:    configuration.readonlyUserID,
				ChannelId: channelID,
				Message:   "Posting is not allowed in this channel.",
			})

			return nil, plugin.DismissPostError
		}
	}

	// Reject posts mentioning the read-only plugin user.
	if strings.Contains(post.Message, fmt.Sprintf("@%s", configuration.Username)) {
		p.API.SendEphemeralPost(post.UserId, &model.Post{
			UserId:    configuration.readonlyUserID,
			ChannelId: post.ChannelId,
			Message:   "Shh! You must not talk about the read-only plugin user.",
		})

		return nil, plugin.DismissPostError
	}

	// Otherwise, allow the post through.
	return post, ""
}
