# fallacy

[![GoDoc](https://godoc.org/github.com/qua3k/fallacy?status.svg)](https://godoc.org/github.com/qua3k/fallacy)

"fallacy bot good" – a logician

## About

Fallacy is a custom high performance Matrix moderation bot written for [Spite]
(https://matrix.to/#spitetech:matrix.org). Born out of the deficiencies of
similar moderation bots, it aims to be simple, fast, and efficient.

Much of the design was inspired by [@rSophieBot](https://t.me/rSophieBot).

## Features

*   Written in Go, making use of the language's native concurrency primitives
*   Intuitive message purging support by replying to the oldest message to
    purge
*   Kicking/banning users via glob, as well as preemptive bans for valid MXID
    literals
*   Pinning messages via reply
*   Prejudiced against Firefox users
*   Automatic joining of upgraded rooms
*   Puppeting via the bot to say messages

## In-progress

*   Muting users via power levels
    *   There are serious problems with the current implementation due to a lack
        of a hash map storing the previous power level; this will be rectified
        soon.
*   Proper room-specific settings
*   Extensive ban-list support including an exception list for handling of admin
    actions
    *   Glob-matching for both disallowed display names and MXIDs including
        differing actions to take for both

## Future

*   Spam/flooding detection
*   Features relying on time
    *   Banning all new users from a specific homeserver for a specified length
        of time
    *   Automatically unmuting users after a specified period of time including
        preventing admins from using it on each other
*   Granular purging support as well as other features only made possible by
    tracking the previous message
*   Promoting and demoting users
*   "Slow mode" or shadowbanning via silently redacting a user's messages if the
    last one was sent within a time window

## Building

Download the Go toolchain from [go.dev](https://go.dev/dl/) and build.

```
go install github.com/qua3k/fallacy/fallacy@latest
```
