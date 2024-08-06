FROM alpine:3.20
COPY bin/thumbla /thumbla
EXPOSE 1323
ENTRYPOINT ["/thumbla"]
