# ddns godaddy on docker
## How to use
[![This image on DockerHub](https://img.shields.io/docker/pulls/awei/godaddy-ddns.svg)](https://hub.docker.com/r/awei/godaddy-ddns/)

[View on Docker Hub](https://hub.docker.com/r/awei/godaddy-ddns)

```shell
$ docker run -d -e GODADDY_KEY=<godaddy-api-key> -e GODADDY_SECRET=<godaddy-api-secret> -e DOMAIN=<your.domain> -e NAME=@ -e TYPE=A -e TTL=600 awei/godaddy-ddns
```

### Necessary Environment Variables
* `GODADDY_KEY` your dodaddy api key (string)
* `GODADDY_SECRET` your dodaddy api srcret (string)
* `DOMAIN` your domain (string)

### Option Environment Variables
* `NAME` DNS Record (string,default: "@")
* `TYPE` DNS record type (default: "A")
* `TTL` TTL (integer,default: 600)
* `CHECK_URL` a url can return your ip (string,default: "http://whatismyip.akamai.com/")
* `CRON_TIME` crontab job time (default: "*/5 * * * *")

