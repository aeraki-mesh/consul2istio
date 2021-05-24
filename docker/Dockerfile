# docker-reg.devops.xiaohongshu.com/shequ/consul2istio:latest

FROM alpine:3.13

ENV consulAddress="127.0.0.1:8500"
ENV consulZone="qc/sh4"

COPY consul2istio /usr/local/bin/
ENTRYPOINT /usr/local/bin/consul2istio --consulAddress=$consulAddress --consulZone=$consulZone