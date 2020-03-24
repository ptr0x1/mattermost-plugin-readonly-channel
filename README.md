# Plugin Read-Only Channel

This plugin was created from the starter plugin template as well as the demo template as a proof-of-concept to create a read-only channel. Once installed, you can configure a channel that will become read-only, and a user that can still post in it.

To learn more about Mattermost plugins, see [our plugin documentation](https://developers.mattermost.com/extend/plugins/).

## Getting Started
Clone the repository.

Note that this project uses [Go modules](https://github.com/golang/go/wiki/Modules). Be sure to locate the project outside of `$GOPATH`, or allow the use of Go modules within your `$GOPATH` with an `export GO111MODULE=on`.

Build your plugin:
```
make
```

This will produce a single plugin file (with support for multiple architectures) for upload to your Mattermost server:

```
dist/com.mattermost.plugin-readonly-channel.tar.gz
```

Deploy and upload your plugin via the [System Console](https://about.mattermost.com/default-plugin-uploads).
