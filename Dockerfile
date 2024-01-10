FROM alpine:3.19
COPY bin/thumbla /thumbla
EXPOSE 1323
ENTRYPOINT ["/thumbla"]
