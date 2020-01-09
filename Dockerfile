FROM nginx:alpine

COPY godaddy-ddns.sh /home/
RUN apk add --update bash curl jq  && \
    chmod -x /home/godaddy-ddns.sh

ENTRYPOINT ["/home/godaddy-ddns.sh"]
