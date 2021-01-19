FROM python:3.8

RUN apt -y update && \
    apt -y install docker.io

COPY requirements.txt /tmp/requirements.txt

RUN pip install -r /tmp/requirements.txt

# required volumes to be mounted at runtime
VOLUME [ "/var/run/docker.sock", "/sys/fs/cgroup", "/tmp/scratch", "/repo" ]

WORKDIR /repo