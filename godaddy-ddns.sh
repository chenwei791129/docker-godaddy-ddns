#!/bin/sh

# GoDaddy.sh v1.3 by Nazar78 @ TeaNazaR.com
###########################################
# Simple DDNS script to update GoDaddy's DNS. Just schedule every 5mins in crontab.
# With options to run scripts/programs/commands on update failure/success.
#
# Requirements:
# - curl CLI - On Debian, apt-get install curl
#
# History:
# v1.0 - 20160513 - 1st release.
# v1.1 - 20170130 - Improved compatibility.
# v1.2 - 20180416 - GoDaddy API changes - thanks Timson from Russia for notifying.
# v1.3 - 20180419 - GoDaddy API changes - thanks Rene from Mexico for notifying.
#
# PS: Feel free to distribute but kindly retain the credits (-:
###########################################
# Modify by AwEi for https://github.com/chenwei791129/docker-godaddy-ddns

# Writable path to last known Public IP record cached. Best to place in tmpfs.
CachedIP=/tmp/current_ip

Curl=$(which curl 2>/dev/null)
[ "${Curl}" = "" ] &&
echo "Error: Unable to find curl CLI." && exit 1
[ -z "${GODADDY_KEY}" ] || [ -z "${GODADDY_SECRET}" ] &&
echo "Error: Requires API Key/Secret value." && exit 1
[ -z "${DOMAIN}" ] &&
echo "Error: Requires Domain value." && exit 1
[ -z "${TYPE}" ] && TYPE=A
[ -z "${NAME}" ] && NAME=@
[ -z "${TTL}" ] && TTL=600
[ "${TTL}" -lt 600 ] && TTL=600
echo -n>>${CachedIP} 2>/dev/null
[ $? -ne 0 ] && echo "Error: Can't write to ${CachedIP}." && exit 1
[ -z "${CHECK_URL}" ] && CHECK_URL=http://api.ipify.org
echo -n "Checking current public IP from ${CHECK_URL}..."
PublicIP=$(${Curl} -kLs ${CHECK_URL})
if [ $? -eq 0 ] && [[ "${PublicIP}" =~ [0-9]{1,3}\.[0-9]{1,3} ]];then
	echo "${PublicIP}!"
else
	echo "Fail! ${PublicIP}"
	eval ${FAILED_EXEC}
	exit 1
fi
if [ "$(cat ${CachedIP} 2>/dev/null)" != "${PublicIP}" ];then
	echo -n "Checking '${DOMAIN}' IP records from GoDaddy..."
	Check=$(${Curl} -kLs -H "Authorization: sso-key ${GODADDY_KEY}:${GODADDY_SECRET}" \
	-H "Content-type: application/json" \
	https://api.godaddy.com/v1/domains/${DOMAIN}/records/${TYPE}/${NAME} \
	2>/dev/null | grep -Eo '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' 2>/dev/null)
	if [ $? -eq 0 ] && [ "${Check}" = "${PublicIP}" ];then
		echo -n ${Check}>${CachedIP}
		echo -e "Unchanged!\nCurrent public IP matches GoDaddy records. No update required!"
	else
		echo -en "changed!\nUpdating '${DOMAIN}'..."
		Update=$(${Curl} -kLs -X PUT -H "Authorization: sso-key ${GODADDY_KEY}:${GODADDY_SECRET}" \
		-H "Content-type: application/json" -w"%{http_code}" -o/dev/null \
		https://api.godaddy.com/v1/domains/${DOMAIN}/records/${TYPE}/${NAME} \
		-d "[{\"data\":\"${PublicIP}\",\"ttl\":${TTL}}]" 2>/dev/null)
		if [ $? -eq 0 ] && [ "${Update}" -eq 200 ];then
		echo -n ${PublicIP}>${CachedIP}
		echo "success!"
		eval ${SUCCESS_EXEC}
		else
		echo "fail! HTTP_ERROR:${Update}"
		eval ${FAILED_EXEC}
		exit 1
		fi
	fi
else
	echo "Current public IP matches cached IP recorded. No update required!"
fi
exit $?
