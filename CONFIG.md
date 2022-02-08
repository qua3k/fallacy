# Configuration

This is the configuration for the fallacy bot.

## Username

The MXID of the client.

```toml
username = "@example:example.com"
```

## Password

The password of the user.

```toml
password = "password"
```

## Homeserver

The homeserver URL. If unspecified, defaults to matrix-client.matrix.org.

```
homeserverUrl = "https://example.com" 
```

## Additional Configuration

### Firefox Harassment

Choose to be extremely prejudiced against Firefox users.

```toml
[Config]
firefox = true # you should probably choose this option
```

### Client Name

The client display name in sessions. Also used for summoning the bot, i.e.,
!fallacy.

```toml
[Config]
name = "fallacy"
```

### Welcome Messages

Whether to welcome new users.

```toml
[Config]
welcome = true
```

## Example Configuration

An example configuration.

```toml
homeserverUrl = "https://example.com"
username = "@fallacy:example.com
password = "ad_hominem"

[Config]
firefox = true
name = "fallacy"
welcome = true
```
