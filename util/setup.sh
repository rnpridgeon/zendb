#! /usr/bin/env bash

self="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $self

export GOPATH=$self

printf "Enter zendesk subdomain: "
read domain

printf "Enter username: "
read user

printf "Enter password: "
read password 

if [ ! -d ../exclude ]; then 
	mkdir ../exclude
fi

cat ../exampleConfig.json | jq ".zendesk.subdomain = \"$domain\"" | jq ".zendesk.user = \"$user\"" | \
	jq ".zendesk.password = \"$password\"" > ../exclude/conf.json


cd ..

./util/initdb.sh -s mysql

go test
