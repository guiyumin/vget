#!/bin/bash
set -e

# Fix ownership of mounted volumes if running as root
if [ "$(id -u)" = "0" ]; then
    chown -R 1000:1000 /home/vget/downloads /home/vget/.config/vget
    exec gosu 1000:1000 vget-server "$@"
else
    exec vget-server "$@"
fi
