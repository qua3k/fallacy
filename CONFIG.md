# Configuration

This is the configuration for the fallacy bot.

```toml
# The username and password used to login to the account
username = "@example:example.com"
password = "password"
# the homeserver URL. If unspecified, is matrix-client.matrix.org
homeserverurl = "https://example.com" 


# Optional configuration, defaults to uninitialized values 
[Config]
# choice to harass firefox users
firefox = true

# the client display name used for calling the bot, i.e., !fallacy
name = "fallacy"
# whether to welcome new users.
welcome = true
```

