# Dockerfile for the Claudia API/web server, and ingest daemon

FROM debian:9.1

RUN apt-get update && apt-get install -y \
    # ca-certificates required for AWS SDK
    ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

EXPOSE 80 443

COPY dist/bin/* /usr/bin/
COPY ui/dist /var/lib/claudia/assets

CMD ["/usr/bin/claudiad"]
