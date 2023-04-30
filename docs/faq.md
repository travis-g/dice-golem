---
title: Dice Golem FAQ
sitemap: false
description: Common questions about Dice Golem, tips, and fixes for issues.
---

# Common Questions

## Why use Dice Golem?

Most Discord dice bots have similar features and commands but accept different dice roll syntaxes. Dice Golem uses a syntax similar to Roll20 and other <abbr title="Virtual Table Tops">VTTs</abbr>' "math expression" syntax, like `3 * 2d6 + 1`.

Dice Golem is easy to use from the moment it's added. It's built to stay out of the way of your games but still feel like a native part of Discord with message context commands and autocomplete suggestions.

User privacy is extremely important: whether you have a public server with 30,000 users or a server just for you and your friends, Dice Golem <u>cannot</u> read server messages unless the messages mention Dice Golem. [Learn more](#what-messages-can-dice-golem-read) or read the bot's [Privacy Policy][privacy].

## How do I roll dice?

The prefered way is with the <span class="mention">/roll</span> command! When making rolls with Dice Golem you provide a dice expression to evaluate, like <span class="param">2d6+4</span>. Your chatbox should look like this:

> **/roll** <span class="param">expression:</span>2d6+4

You can add other options too, like a <span class="param">label</span>:

> **/roll** <span class="param">expression:</span>2d20kh1+3 <span class="param">label:</span>Lucky initiative <span class="param">secret:</span>True

Other Slash commands like <span class="mention">/secret</span> make rolls with certain options implied.

## Slash commands aren't working

Sorry! You can [@mention Dice Golem as a workaround](#how-do-i-roll-dice-without-slash-commands), but there are some things you can check to troubleshoot:

- Check that you have **Use Application Commands** permission where you want to use the bot (server owners have this permission everywhere).
- Check that Dice Golem's commands are configured in the desired channel: a server manager can browse **Integrations > Dice Golem** to change command and role permissions.
- Turn on typing suggestions for Slash Commands: right-click the chat box and enable **Suggestions > Slash Commands**. You can use apps and commands manually from the chat box's <span class="param">➕</span> button menu.
- Open a direct message with Dice Golem and to see if commands can be used there: almost all of the bot's Slash commands should be available.
- Try using the bot from a different device or the browser: if commands work for one device but not another there may be an issue with the Discord client.
- Make sure your Discord client is up to date by checking for updates to your app. Slash commands are still evolving and old Discord versions don't support them.
- Make sure you're using Discord's "new" chat input: check **Settings > Accessibility > Chat Input** and disable **Use the legacy chat input**. Discord's legacy chat input does not support slash commands, but users that need the legacy input for accessibility can use the bot with [@mentions](#how-do-i-roll-dice-without-slash-commands).

In rare cases there could be other problems or an issue with Discord. Please ask in the [support server][support]!

## How do I roll dice without Slash commands?

Dice Golem responds to messages that @mention it. It tries to roll the message's other text: <span class="param"><span class="mention">@Dice Golem</span> 3d6+3</span> will roll `3d6+3`.

You can add labels inline after a <kbd>#</kbd> or <kbd>\</kbd>: <span class="param"><span class="mention">@Dice Golem</span> 4d8 # fire damage!</span>

## What messages can Dice Golem read?

Dice Golem <u>cannot</u> read messages sent in servers unless the messages @mention it. Dice Golem can read:

- Server messages that @mention Dice Golem,
- Messages sent directly to it through DMs,
- Messages forwarded via message commands (ex. <span class="mention">Roll Message</span>).

[privacy]: https://dicegolem.io/privacy "Dice Golem's Privacy Policy"
[support]: https://discord.gg/vuE8zyc "The Pit of Dicepair, Dice Golem's support server"
