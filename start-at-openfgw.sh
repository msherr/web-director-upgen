#!/bin/bash
if [ $UID -ne "0" ]; then
	echo "be root"
	exit 1
fi
SERVER_AUTH_TOKEN=krjci4k5xlfkmafdkrt,gfgklfa ./server -certpath /etc/letsencrypt/live/opengfw.cs-georgetown.net/fullchain.pem -keypath /etc/letsencrypt/live/opengfw.cs-georgetown.net/privkey.pem -port 443 -user msherr
