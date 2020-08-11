# helpbot

A discord bot to help teaching assistants keep track of students who need help.

This bot manages a queue of students who request help from teaching assistants.
In the future, the bot will also be able to integrate with [autograder](https://github.com/autograde/quickfeed).

## Planned features

* Automatic user registration using GitHub and Autograder
* Message updates when position in queue changes
* Filter by what type of help is needed, e.g. regular "help" or getting "approval" on an assignment.

## Current features

* Students can request help using "!gethelp" or request approval of an assignment using "!approve"
* Students can cancel their request using "!cancel"
* Students receive a message when a teaching assistant is ready to help.

* Teaching assistants can get the next help request using "!next"
  * If the queue is empty, the teaching assistant will receive a notification when the next student requests help.
* Teaching assistants can view the queue.
* Teaching assistants can clear the queue.

## Work in progress

* Registration is halfway implemented:
  * Currently, new users can become "registered" by using the command "!register" and typing their GitHub username.
    The bot will then check if that GitHub username has access to the course's GitHub organization.
  * In the future, this should be properly authenticated using Oauth either using the GitHub API, or by using Discord connections.
  * Also, autograder integration is planned, such that the student's real names and student ids can be retrieved.
  * When the bot has confirmed that the user has access to GitHub/autograder, the bot will automatically assign a nickname and roles.

## Setup

Configuration is done either through environment variables, command line flags, or a config file.
You may mix and match these.

### Config file

A config file must be named `.helpbotrc`. The default syntax is TOML.
You may specify a different file extension, for example ".yml" or ".json" if you prefer a different markup language.

The following configuration variables must be set:

```toml
db-path = "<path to sqlite database file>"
token = "<discord bot token>"
prefix = "!" # the prefix before each command
guild = <discord id>
help-channel = <discord id>
lobby-channel = <discord id>
student-role = <discord id>
assistant-role = <discord id>
gh-token = "<github access token>"
gh-org = "<github org name>"
```
