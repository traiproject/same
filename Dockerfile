# Dockerfile
FROM alpine
COPY same /usr/bin/same
ENTRYPOINT ["/usr/bin/same"]
