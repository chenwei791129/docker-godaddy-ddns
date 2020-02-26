# ddns godaddy on docker
## How to use
[![This image on DockerHub](https://img.shields.io/docker/pulls/awei/godaddy-ddns.svg)](https://hub.docker.com/r/awei/godaddy-ddns/)

[View on Docker Hub](https://hub.docker.com/r/awei/godaddy-ddns)

```shell
$ docker run -d -e GODADDY_KEY=<godaddy-api-key> -e GODADDY_SECRET=<godaddy-api-secret> -e DOMAIN=<your.domain> -e NAME=@ -e TTL=600 awei/godaddy-ddns
```

### Necessary Environment Variables
* `GODADDY_KEY` your dodaddy api key (string)
* `GODADDY_SECRET` your dodaddy api srcret (string)
* `DOMAIN` your domain (string)

### Option Environment Variables
* `NAME` DNS Record (string,default: "@")
* `TTL` TTL (integer,default: 600)
* `CHECK_URL` a url can return your ip (string,default: "http://whatismyip.akamai.com/")
* `CRON_TIME` crontab job time (default: "*/5 * * * *")

## Code Source
- [GoDaddy.sh v1.0 by Nazar78 @ TeaNazaR.com](http://teanazar.com/2016/05/godaddy-ddns-updater/) Thanks Nazar78 for work :)

## License
The repository is open-sourced software licensed under the [MIT license](https://opensource.org/licenses/MIT).
