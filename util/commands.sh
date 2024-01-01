#!/usr/bin/env sh
curl -s -H "Authorization: Bot $BOT_TOKEN" https://discord.com/api/v10/applications/581956766246633475/commands | jq '[.[]|select(.type==1)| {"name","description","options"}]'
