# Usage Guide

## Table of Contents

*   [Banning Users](#banning-users)
*   [Muting/Unmuting Users (INCOMPLETE)](#mutingunmuting-users-incomplete)
*   [Pinning Messages](#pinning-messages)
*   [Purging Messages](#purging-messages)
*   [Sockpuppet Functionality](#sockpuppet-functionality)

## Banning Users

fallacy features functionality to ban MXIDs or globs of MXIDs.

**Format:**
```
    !fallacy ban <glob>
```

If the supplied glob is a literal MXID, it will resort to preemptively banning
the user rather than iterating over the members list.

## Muting/Unmuting Users (INCOMPLETE)

fallacy features incomplete functionality to mute/unmute users.

This feature is seriously flawed due to how it works. fallacy must use power
levels to demote/promote users to properly prevent them from sending messages;
when used on an admin it renders them unable to unmute themselves or use their
moderation tools, resulting in disastrous consequences. This could be
potentially mitigated by persisting the user's previous power level to a
database and restoring it on unmute (non-admins) and/or refusing to mute admins
altogether.

**Formats**
```
    !fallacy mute <mxid>
    !fallacy unmute <mxid>
```

## Pinning Messages

fallacy features functionality to pin messages.

**Format**
```
    !fallacy pin
```

Pins the message you replied to, otherwise makes angry noises.

## Purging Messages

fallacy features the ability to purge messages, for the good of mankind.

**Formats:**
```
    !fallacy purge
    !fallacy purge <mxid> <int>
```

### Function

This command can be used two ways:
1.  replying and deleting messages from all users
1.  deleting messages from a specific user, with optional limit

The first option deletes all messages newer and including the message you
replied to. This can be effectively demonstrated with a simple example.
```
    <duck> 1:11:11 hello
    <duck> 1:11:12 spam
    <duck> 1:11:13 spam
    <duck> 1:11:14 spam
    <the admin should reply to the first message (hello) with "!fallacy purge">
```

The second option allows you to delete all message from a specific user, with an
optional limit on the messages to purge. Omission of the limit is understood to
mean to purge all messages from that user.

## Sockpuppet Functionality

fallacy features the functionality to allow any admin to use the bot to
communicate a message.

**Format**
```
    say <text>
```
