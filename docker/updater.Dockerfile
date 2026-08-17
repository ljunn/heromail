FROM docker:27-cli

RUN apk add --no-cache docker-cli-compose
COPY docker/updater.sh /usr/local/bin/heromail-updater
RUN chmod +x /usr/local/bin/heromail-updater
ENTRYPOINT ["/usr/local/bin/heromail-updater"]
