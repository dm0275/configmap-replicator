FROM alpine

COPY ./build/configmap-replicator-operator-linux-amd64 .

CMD ./configmap-replicator-operator