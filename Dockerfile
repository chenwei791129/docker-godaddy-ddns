ARG BASE_IMAGE=alpine
FROM ${BASE_IMAGE}:3.16.3

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
    chmod +x /scripts/godaddy-ddns.sh

CMD echo "${CRON_TIME} /scripts/godaddy-ddns.sh" >> /etc/crontabs/root && crond -f -L 2
