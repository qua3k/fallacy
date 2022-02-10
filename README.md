# fallacy

[![GoDoc](https://godoc.org/github.com/qua3k/fallacy?status.svg)](https://godoc.org/github.com/qua3k/fallacy)

"fallacy bot good" – a logician

## About

Fallacy is a high performance Matrix moderation bot. Born out of the
deficiencies of other matrix bots, it aims to be simple, fast, and efficient.

Much of the design was inspired by [@rSophieBot](https://t.me/rSophieBot).

Currently, it is dependent on rate-limits being disabled, but it is possible to
rework many of the commands to cope with the rate limits. It is worth noting
this bot is not suited for homeservers unable to handle more than a couple
requests at a time, especially when purging vast amounts of messages.

## Features

*   written in go which is supposed to be good
    *   abuses goroutines to hog your system's resources
*   follows you everywhere you go (room upgrades)
*   actually thanks users for coming (welcome messages)
*   is prejudiced against firefox users
*   (in progress) mjolnir banlist support

## Commands

*   tells users you dont want to hear them (mute users)
*   removes your shitposts (purges messages)
*   lets everyone know how cool you are (pin messages)
*   sprouts fallacies, if you choose to (say messages)
*   removes your enemies (bans users)

## To-do

*   automatically unmute users after a certain period of time
*   kicking/banning certain display and usernames
*   explore banning new users from a certain server for a specific period
*   promote/demote users

## Future

*   slow mode via redacting messages if last one was sent in x time
    *   must evaluate the feasibility of this