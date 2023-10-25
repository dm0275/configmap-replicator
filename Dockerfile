FROM alpine

COPY ./build/configmap-replicator-operator .

CMD ./configmap-replicator-operator