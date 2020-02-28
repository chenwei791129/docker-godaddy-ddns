FROM alpine:latest

ENV GODADDY_KEY= \
    GODADDY_SECRET= \
    DOMAIN= \
    TYPE=A \
    NAME=@ \
    TTL=600 \
    CHECK_URL=http://whatismyip.akamai.com/ \
    CRON_TIME="*/5 * * * *"

WORKDIR /scripts

COPY godaddy-ddns.sh /scripts

RUN apk add --update bash curl jq  && \
    chmod +x /scripts/godaddy-ddns.sh && \
    echo "${CRON_TIME} /scripts/godaddy-ddns.sh" >> /etc/crontabs/root

CMD crond -f -L 2
