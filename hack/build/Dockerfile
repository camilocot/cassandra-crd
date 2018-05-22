FROM alpine:3.6

RUN apk add --no-cache ca-certificates

ADD _output/bin/cassandra-crd /usr/local/bin/cassandra-crd

RUN adduser -D cassandra-crd
USER cassandra-crd
