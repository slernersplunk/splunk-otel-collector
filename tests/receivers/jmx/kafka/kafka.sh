#!/usr/bin/env bash

ZOOKEEPER=$(docker run -d --hostname zookeeper --name zookeeper zookeeper:3.5)
KAFKA=$(docker run -d --hostname kafka -e KAFKA_CFG_ZOOKEEPER_CONNECT=zookeeper:2181 -e ALLOW_PLAINTEXT_LISTENER=yes -e JMX_PORT=7199 -e KAFKA_OPTS='-Djava.rmi.server.hostname=kafka -Dcom.sun.management.jmxremote.rmi.port=7199' -p 7199:7199 --name kafka bitnami/kafka:latest)

docker network connect kafnet $ZOOKEEPER
docker network connect kafnet $KAFKA

docker ps
