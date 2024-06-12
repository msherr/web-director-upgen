#!/bin/bash
if [ $UID -ne "0" ]; then
	echo "be root"
	exit 1
fi
SERVER_AUTH_TOKEN=krjci4k5xlfkmafdkrt,gfgklfa ./server -certpath certs/sad-censored-vm/fullchain.pem -keypath certs/sad-censored-vm/privkey.pem -port 443

