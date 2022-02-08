# Configuration

This is the configuration for the fallacy bot.

```toml
# The username and password used to login to the account.
username = "@example:example.com"
password = "password"
# The homeserver URL. If unspecified, defaults to matrix-client.matrix.org.
homeserverurl = "https://example.com" 


# Optional configuration, defaults to uninitialized values.
[Config]
# Choice to harass firefox users.
firefox = true

# The client displays the name used for calling the bot, i.e., !fallacy
name = "fallacy"
# Whether to welcome new users.
welcome = true
```

