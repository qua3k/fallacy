# Usage Guide

## ban

    ban <glob>

Bans a user with an MXID matching specified glob format. It could potentially be
improved in the future to allow preemptive banning rather than iterating through
the joined members list.

## mute/unmute

    mute <mxid>
    unmute <mxid>

Mutes or unmutes a user by utilizing power levels. The current unmuting
mechanism is flawed as it neglects to store the previous power level of the user,
only restoring them to the power level necessary to speak.

## pin

    pin

Pins the message you replied to, otherwise makes angry noises.

## purge

    purge

Deletes all messages newer than the message you replied to.

## purgeuser

    purgeuser <mxid>

Deletes all messages from a specified user.

## say

    say <text>

The bot says something.
