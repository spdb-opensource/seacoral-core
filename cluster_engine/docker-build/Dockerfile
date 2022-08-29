FROM dbscale/dbscale-kube-base:2104-centos7.8-amd64

ENV LANG C.UTF-8

COPY bin/ /opt/cluster_engine/bin
COPY script /opt/cluster_engine/script

RUN chmod -R 755 /opt/cluster_engine/bin && \
    chmod -R 755 /opt/cluster_engine/script && \
    rm -rf /etc/ansible && \
    ln -s /opt/cluster_engine/script/host-init /etc/ansible

CMD ["/bin/bash"]
