# docker-godaddy-ddns
ddns godaddy on docker
```console
$ docker run -d -e GODADDY_KEY=<godaddy-api-key> -e GODADDY_SECRET=<godaddy-api-secret> -e DOMAIN=<your.domain> -e NAME=@ -e TYPE=A -e TTL=600 awei/godaddy-ddns
```
