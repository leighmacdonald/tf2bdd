# TF2BDd

Very simple service to allow tf2 bot detector player list contributions over discord and serve
the results over a HTTP service to be consumed by any bot detector compatible client.

## Usage

    $ git clone git@github.com:leighmacdonald/tf2bdd.git
    $ cd tf2bdd
    $ go build
    $ export STEAM_TOKEN=steam_web_api_token  # Your steam api key, for resolving vanity names
    $ export BOT_TOKEN=discord_bot_token      # Your discord bot token
    $ export BOT_CLIENTID=12345               # Discord client id
    $ export BOT_ROLES=11111111111,222222222      # Roles allowed to use non-readonly commands
    $ ./tf2bdd
  
## Commands

Bot command list:

- `!add <steamid/profile> [attributes]` Add the user to the master ban list. Valid attributes are 0 or more of: `racist sus/suspicious cheater exploiter`. If none are defined, it will use cheater by default.
- `!del <steamid/profile>` Remove the player from the master list
- `!check <steamid/profile>` Checks if the user exists in the database
- `!count` Shows the current count of players tracked
- `!import <attach_a_json_file>` Imports the steam ids from a players custom ban list
- `!steamid <steamid/profile>` Get the various steam ids
  


## Docker Example

    docker run --rm --name tf2bdd -it \
        -p 127.0.0.1:8899:8899 \
        --env BOT_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
        --env STEAM_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
        --env BOT_CLIENTID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
        --env BOT_ROLES=111111111111,22222222222 \
        --mount type=bind,source="$(pwd)"/.db.sqlite,target=/app/db.sqlite \
        ghcr.io/leighmacdonald/tf2bdd:latest