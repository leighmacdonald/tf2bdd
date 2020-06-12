# TF2BDd

Simple service to send new player lists to the bot detector.

## Usage

    $ git clone git@github.com:leighmacdonald/tf2bdd.git
    $ cd tf2bdd
    $ go build
    $ export STEAM_TOKEN=steam_web_api_token
    $ export BOT_TOKEN=discord_bot_token
    $ ./tf2bdd
  
## Commands


  `!add <steamid/profile> [attributes]` Add the user to the master ban list. Valid attributes are 0 or more of: `racist sus/suspicious cheater exploiter`. If none are defined, it will use cheater by default.
  
  
