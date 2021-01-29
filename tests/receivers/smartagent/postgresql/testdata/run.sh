#!/usr/bin/env bash

docker run -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=test_db \
-v /home/rmfitzpatrick/servizes/postgres/initdb.d:/docker-entrypoint-initdb.d:ro \
-p 5432:5432 \
postgres:latest -c shared_preload_libraries=pg_stat_statements
