#!/bin/sh

# insert user ID into /etc/passwd
# in OpenShift, the UID will be generated
if ! whoami &>/dev/null; then
  if [ -w /etc/passwd ]; then
    echo "${USER_NAME:-appdynamics-operator}:x:$(id -u):$(id -g):${USER_NAME:-appdynamics-operator} user:${HOME}:/sbin/nologin" >> /etc/passwd
  fi
fi

exec "${OPERATOR}" "$@"