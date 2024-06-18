#!/bin/bash
if [ $UID -ne "0" ]; then
	echo "be root"
	exit 1
fi
mkdir -p exp
cd exp
SERVER_AUTH_TOKEN=krjci4k5xlfkmafdkrt,gfgklfa ../server -certpath /etc/letsencrypt/live/sad-censored-vm.cs-georgetown.net/fullchain.pem -keypath /etc/letsencrypt/live/sad-censored-vm.cs-georgetown.net/privkey.pem -port 443 -user ms2382_georgetown_edu


