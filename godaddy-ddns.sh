#!/bin/bash

# GoDaddy.sh v1.0 by Nazar78 @ TeaNazaR.com
###########################################
# Simple DDNS script to update GoDaddy's DNS. Just schedule every 5mins in crontab.
# With options to run scripts/programs/commands on update failure/success.
#
# Requirements:
# - Bash - On LEDE/OpenWRT, opkg install bash
# - curl CLI - On Debian, apt-get install curl
# - jq - On Debian, apt-get install jq
#
# History:
# v1.0 - 20160513 - 1st release.
#
# PS: Feel free to distribute but kindly retain the credits (-:
###########################################
# Modify by AwEi for https://github.com/chenwei791129/docker-godaddy-ddns

touch /tmp/current_ip
echo -n "Checking current 'Public IP' from '${CHECK_URL}'..."
PublicIP=$(curl -kLs ${CHECK_URL})
if [ $? -eq 0 ] && [[ "${PublicIP}" =~ [0-9]{1,3}\.[0-9]{1,3} ]];then
  echo "${PublicIP}!"
else
  echo "Fail! ${PublicIP}"
  exit 1
fi
if [ "$(cat /tmp/current_ip 2>/dev/null)" != "${PublicIP}" ];then
  echo -n "Checking '${DOMAIN}' IP records from 'GoDaddy'..."
  Check=$(curl -kLsH"Authorization: sso-key ${GODADDY_KEY}:${GODADDY_SECRET}" \
  -H"Content-type: application/json" \
  https://api.godaddy.com/v1/domains/${DOMAIN}/records/${TYPE}/${NAME} \
  2>/dev/null|jq -r '.[0].data'>/dev/null)
  if [ $? -eq 0 ] && [ "${Check}" = "${PublicIP}" ];then
    echo -n ${Check}>/tmp/current_ip
    echo -e "unchanged!\nCurrent 'Public IP' matches 'GoDaddy' records. No update required!"
  else
    echo -en "changed!\nUpdating '${DOMAIN}'..."
    Update=$(curl -kLsXPUT -H"Authorization: sso-key ${GODADDY_KEY}:${GODADDY_SECRET}" \
    -H"Content-type: application/json" \
    https://api.godaddy.com/v1/domains/${DOMAIN}/records/${TYPE}/${NAME} \
    -d "[{\"data\":\"${PublicIP}\",\"ttl\":${TTL}}]" 2>/dev/null)
    if [ $? -eq 0 ] && [ "${Update}" = "" ];then
      echo -n ${PublicIP}>/tmp/current_ip
      echo "Success!"
    else
      echo "Fail! ${Update}"
      exit 1
    fi
  fi
else
  echo "Current 'Public IP' matches 'Cached IP' recorded. No update required!"
fi
exit $?
