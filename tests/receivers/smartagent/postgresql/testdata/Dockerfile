ARG POSTGRES_VERSION=11
FROM postgres:${POSTGRES_VERSION}

COPY initdb.d /docker-entrypoint-initdb.d
COPY init.sh /docker-entrypoint-initdb.d/

CMD ["postgres", "-c", "shared_preload_libraries=pg_stat_statements"]
