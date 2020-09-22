FROM alpine

LABEL name="AppDynamics ClusterAgent Operator" \
      vendor="AppDynamics" \
      version="0.6.0" \
      release="1" \
      url="https://www.appdynamics.com" \
      summary="AppDynamics monitoring solution for applications deployed to Kubernetes clusters" \
      description="The ClusterAgent monitors state of Kuberenetes resources and derives metrics to provide visibility into common application impacting issues."


RUN apk upgrade --update --no-cache

ENV OPERATOR=appdynamics-operator \
    USER_UID=1001 \
    USER_NAME=appdynamics-operator


ADD build/_output/bin/appdynamics-operator /usr/local/bin/appdynamics-operator
ADD LICENSE /licenses/
ADD build/bin /usr/local/bin

RUN  /usr/local/bin/ensure-user.sh

ENTRYPOINT ["/usr/local/bin/start.sh"]

USER ${USER_UID}
