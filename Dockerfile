FROM centurylink/ca-certs
COPY bin/thumbla /thumbla
COPY ./config-prod.yml /config-prod.yml
ENTRYPOINT ["/thumbla", "--config", "./config-prod.yml"]
