#!/bin/bash

if [ -z "$EXPNAME" ]; then
    echo "set EXPNAME"
    exit 1
fi

SERVER_AUTH_TOKEN=krjci4k5xlfkmafdkrt,gfgklfa ./director -gfw_url https://opengfw.cs-georgetown.net -censoredvm_url https://sad-censored-vm.cs-georgetown.net -bridge_url https://spare.cs-georgetown.net -bridge_ip 10.128.0.50 -exp $EXPNAME
