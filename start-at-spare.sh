#!/bin/bash

USER=msherr

if [ $UID -ne "0" ]; then
	echo "be root"
	exit 1
fi
mkdir -p exp
chown $USER exp
cd exp
SERVER_AUTH_TOKEN=krjci4k5xlfkmafdkrt,gfgklfa ../server -certpath /etc/letsencrypt/live/spare.cs-georgetown.net/fullchain.pem -keypath  /etc/letsencrypt/live/spare.cs-georgetown.net/privkey.pem -port 443 -user msherr


