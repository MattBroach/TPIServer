# Twitch Plays Itself Server

Software for storing centralized filter data for [Twitch Plays (With) Itself](https://www.twitch.tv/twitch_plays_itself).  Handles two related tasks:

1. Ingests current filter values (as HTTP POST requests) from the Twitch Plays Itself video processor.
2. Feeds current filter values (as websockets) to the panel extensions
