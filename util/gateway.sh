#!/usr/bin/env sh
# https://discord.com/developers/docs/topics/gateway#get-gateway-bot
curl -s -H "Authorization: Bot $BOT_TOKEN" https://discord.com/api/v10/gateway/bot | jq .
