#!/usr/bin/env sh
# https://discord.com/developers/docs/interactions/application-commands#get-global-application-commands
curl -s -H "Authorization: Bot ${GOLEM_API_TOKEN}" https://discord.com/api/v10/applications/${GOLEM_ID}/commands \
| jq '.[] |= del(.id,.version,.nsfw,.application_id,.dm_permission)' \
| yj -y
