---
title: FAQ
sitemap: false
description: Common questions about Dice Golem, tips, and fixes for issues.
---

# FAQ

## Why use Dice Golem?

Dice Golem is easy to use from the moment it's added and takes advantage of newer bot capabilities of Discord: message commands, autocomplete suggestions, buttons, and more! It's built to stay out of the way of your games while still feeling like a native part of Discord. You can also install it directly to your Discord account and use it anywhere!

Most Discord dice bots have similar features and commands but accept different dice roll syntaxes. Dice Golem uses a syntax similar to Roll20 and other <abbr title="Virtual Table Tops">VTTs</abbr>' "math expression" syntax, like `3 * 2d6 + 1`.

User privacy is extremely important: whether you're in a public server with 30,000 users or a server just for you and your friends, Dice Golem <u>cannot</u> read server messages unless the messages mention Dice Golem. [Learn more](#what-messages-can-dice-golem-read) or read the bot's [Privacy Policy][privacy].

## How do I roll dice?

The prefered way is with the <span class="mention">/roll</span> command! When making rolls with Dice Golem you provide a dice expression to evaluate, like <span class="param">2d6+4</span>. Your chatbox should look like this:

> **/roll** <span class="param">expression:</span>2d6+4

You can add other options too, like a <span class="param">label</span>:

> **/roll** <span class="param">expression:</span>2d20kh1+3 <span class="param">label:</span>Lucky initiative <span class="param">secret:</span>True

Other Slash commands like <span class="mention">/secret</span> make rolls with certain options implied.

## Slash commands aren't working

Sorry! The bot will likely still respond to DMed rolls and [@mentions as a workaround in servers](#how-do-i-roll-dice-without-slash-commands), but there are some things you can do to troubleshoot:

- Check that Dice Golem's user appears Online in your server's members list. If it is offline, please check the [support server][support]. Thank you!
- Check that you have _Use Application Commands_ permission where you want to use the bot (server owners have this permission everywhere).
- Check that Dice Golem's commands are configured in the desired channel: a server manager can browse **Integrations > Dice Golem** to change command and role permissions.
- Turn on typing suggestions for Slash commands: right-click the chat box and enable **Suggestions > Slash commands**. You can use apps and commands manually from the chat box's <span class="param">＋</span> button.
- Open a direct message with Dice Golem and to see if commands can be used there: almost all of the bot's Slash commands should be available.
- Try using the bot from a different device or a web browser: if commands work for one device but not another there may be an issue with the Discord client.
- Make sure your Discord client is up to date by checking for updates. Slash commands are still evolving and old Discord versions don't support them.
- Make sure you're using Discord's "new" chat input: check **Settings > Accessibility > Chat Input** and disable **Use the legacy chat input**. Discord's legacy chat input does not support Slash commands, but users that need the legacy input for accessibility can use the bot with [@mentions](#how-do-i-roll-dice-without-slash-commands).

In rare cases there could be other problems or an issue with Discord. Please ask in the [support server][support]!

## How do I roll dice without Slash commands?

Dice Golem responds to messages that @mention it in servers and in direct message channels where have the bot user added. It tries to roll the message's other text: <span class="param"><span class="mention">@Dice Golem</span> 3d6+3</span> will roll `3d6+3`. The bot user will need _Send Messages_ permission to respond.

You can add labels inline after a <kbd>#</kbd> or <kbd>\</kbd>: <span class="param"><span class="mention">@Dice Golem</span> 4d8 # fire damage!</span>

## What messages can Dice Golem read?

Dice Golem can read:

- Server messages that @mention Dice Golem,
- Messages sent directly to it through a DM to the bot user,
- Messages forwarded via message commands (ex. <span class="mention">Roll Message</span>).

When using the bot's Slash or Message commands it is only ever sent the data you enter or the message you call it against. Dice Golem <u>cannot</u> read messages sent in servers unless the messages @mention it.

## How do I remove the bot?

If added to a server, browse to **Server Settings > Integrations > Dice Golem** and click **Remove App** at the bottom. If the bot user remains, it can be kicked.

If installed to your user, browse to **User Settings > Authorized Apps**, locate Dice Golem in the list, and click **Deauthorize**.

[privacy]: https://dicegolem.io/privacy "Dice Golem's Privacy Policy"
[support]: https://discord.gg/vuE8zyc "The Pit of Dicepair, Dice Golem's support server"
