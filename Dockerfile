ARG BASE_IMAGE=alpine
FROM ${BASE_IMAGE}:3.16.4

ENV GODADDY_KEY= \
    GODADDY_SECRET= \
    DOMAIN= \
    TYPE=A \
    NAME=@ \
    TTL=600 \
    CHECK_URL=http://whatismyip.akamai.com/ \
    CRON_TIME="*/5 * * * *" \
    SUCCESS_EXEC="" \
    FAILED_EXEC=""

WORKDIR /scripts

COPY godaddy-ddns.sh /scripts

RUN apk add --update curl && \
    chmod +x /scripts/godaddy-ddns.sh

CMD echo "${CRON_TIME} /scripts/godaddy-ddns.sh" >> /etc/crontabs/root && crond -f -L /dev/null
