
#!/bin/sh
set -x

mkdir -p "${HOME}"
chown "${USER_UID}":0 "${HOME}"
chmod ug+rwx "${HOME}"

chmod g+rw /etc/passwd

# clean-up
rm "$0"