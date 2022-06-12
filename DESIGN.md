# Design

This is a design document detailing some of the considered design choices behind
the fallacy bot, in the hopes that it will help future bot writers.

## Banlists (join rules)

There is a need for moderators to perform moderations on joined users based on
their MXID or display name, and it is expected that the fallacy bot will
accomplish this task. Firstly, we will talk about the MXID filtering, and how
it is implemented.

Early on in the design process for this feature, the matrix.org team had already
introduced the concept of moderation policy lists, and we decided that instead
of creating a bot-specific way of accomplishing the same goal, we would
consolidate the join rules functionality into the policy lists.

Each room has an entry in the database, with a column of rules (the value of the
`entity` key of moderation policy events), and a column of subscribed rooms.
"Subscribing" to these moderation policy lists entails joining the bot to the
room with the list and adding the subscribing room to the `subscribed_rooms`
column of the room in the database.

```json
{
  "content": {
    "entity": "@alice*:example.org",
    "reason": "undesirable behaviour",
    "recommendation": "m.ban"
  },
  "event_id": "$143273582443PhrSn:example.org",
  "origin_server_ts": 1432735824653,
  "room_id": "!jEsUZKDJdhlrceRyVU:example.org",
  "sender": "@example:example.org",
  "state_key": "rule:@alice*:example.org",
  "type": "m.policy.rule.user",
  "unsigned": {
    "age": 1234
  }
}
```

*The typical format for a moderation policy event. The state_key field is not
required to be any specific value and is therefore of no utility.*

Once the bot receives an moderation policy event, the content of `entity` is
appended to that room's `user_rules` array. The bot then iterates through all
the subscribed rooms and attempts to take action based on their specific
preference (more on that below), providing the baseline moderation policy list
support.

Each room can set a global preference on possible actions to take when a user in
their room is affected by moderation policy lists. If the fallacy bot is
missing permissions in the protected room it might just scream and say who
needs to be banned :)

| Int   | Action    |
|-----  |--------   |
| 0     | none      |
| 1     | kick      |
| 2     | ban       |

*Room admins can configure one of three possible actions to take when a user is
affected by a policy ban as seen in the above table -- the default is to ban.*

### Exceptions

The fallacy bot will also feature the possibility of exceptions. If a user is
banned/kicked using the banlist functionality, but an admin manually changes
their status (either by unbanning them or using the `ignore` functionality --
they are added to the exception list (a hash map structure stored to the
database). Banning them again results in them being removed from the map and
marked eligible for a ban via moderation policy lists.

### Cleanup

The bot will automatically walk up the timeline and redact any events sent by
the offending parties, just in case :)

### Display Names

There is also the need to perform moderation actions on certain display names:
this can be easily added to the bot by creating a `display_rules` column in the
database and a `display_action` following the `action` schema. We sense that
offending display names may need to be treated differently as they may be
enforced on community members on display name changes rather than strictly new
joins like MXID filtering. Using the `ignore` command or having an admin change
their membership status will also add them to the exception list.

## Power Levels

As the fallacy bot relies on power levels for nearly all of its functionality,
it makes several unnecessary requests for power levels, even when the power
levels have not changed between its last request. The solution is to store the
most recent power levels event as JSON in the database, updating accordingly
when the bot receives a new power levels event or when a room member issues the
`levels` command. Instead of possibly locking out potential admins, we elected
to update the power levels even when the requesting party is not an admin. An
attacker could potentially leverage this to cause numerous unwanted updates,
but we felt that this was a necessary evil.

However, this may still be subject to change in the future.

## Bot Admin(s)

This would be a set of users that would be able to configure bot-specific
settings in any room they share with the bot, such as modifying the
configuration on the fly (wanted for a future feature). Bot admins should be
able to add other bot admins without restarting the bot.

## Dynamic Configuration

We can modify the read string slice in-memory, but we want to be able to persist
that configuration change to disk. This will involve checking for the existence
of the TOML key, finding the ending bracket (`]`), checking for the existence
of a trailing comma, and appending it to the array. This should be disabled by
default, as I'm not entirely confident in the parsing :)

## Restricted Mode

fallacy was designed around the notion of a public service bot akin to other
Telegram moderation bots, and as such has made numerous design decisions such
as foregoing Mjolnir's privileged moderation room, instead only checking power
levels. This makes it difficult to self-host for admins who are adverse to
others using their bot, and as such, the bot ignores invites by default. This
is changing with the addition of restricted mode, an additional option in the
TOML config (`permitted_rooms`) that allows bot admins to specify which rooms
the bot will operate in, ignoring all other rooms.
