# docker-godaddy-ddns
ddns godaddy on docker
```console
$ docker run --rm awei/godaddy-ddns --key=<godaddy-api-key> --secret=<godaddy-api-secret> --domain=<your.domain> --record=@ --type=A --ttl=600
```
