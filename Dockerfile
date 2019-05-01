from ubuntu:18.04

RUN apt-get update && apt-get install -y openssl uuid-runtime

COPY ota-kube /usr/local/bin
COPY create-device.sh /usr/local/bin

WORKDIR /data

COPY certs /data/certs

ENV DATA_PATH=/data