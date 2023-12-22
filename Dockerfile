FROM alpine

COPY ./build/configmap-replicator-linux-amd64 configmap-replicator

CMD ./configmap-replicator