FROM alpine:latest

WORKDIR /scripts

COPY godaddy-ddns.sh ./

RUN apk add --update bash curl jq  && \
    chmod -x /scripts/godaddy-ddns.sh

ENTRYPOINT ["/home/godaddy-ddns.sh"]
