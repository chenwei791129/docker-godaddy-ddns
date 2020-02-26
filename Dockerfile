FROM alpine:latest

ENV GODADDY_KEY= \
    GODADDY_SECRET= \
    DOMAIN= \
    NAME=@ \
    TTL=600 \
    CHECK_URL=http://whatismyip.akamai.com/ \
    CRON_TIME="*/5 * * * *"

WORKDIR /scripts

COPY godaddy-ddns.sh /scripts

RUN apk add --update bash curl jq dcron  && \
    chmod +x /scripts/godaddy-ddns.sh

RUN echo "${CRON_TIME} /scripts/godaddy-ddns.sh>>/var/log/godaddy-ddns.log" >> /etc/crontabs/root
RUN touch /var/log/godaddy-ddns.log
CMD crond -f && tail -f /var/log/godaddy-ddns.log
