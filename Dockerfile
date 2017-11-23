FROM centurylink/ca-certs
COPY bin/thumbla /thumbla
EXPOSE 1323
ENTRYPOINT ["/thumbla"]
