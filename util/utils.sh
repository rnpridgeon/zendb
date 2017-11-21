#!/usr/bin/env bash

#clean-up if the script is exited for any reason
#trap clean EXIT

#non-interactive scripts won't expand aliases by default
shopt -s expand_aliases

# require(void): Ensures dependencies are on the PATH variable 
require() {
	echo "Checking dependencies ..."
	for var in "$@"; do 
		"$var" &> /dev/null
		if [ $? -eq 127 ]; then
			ERR="Error: $var required but not found, please install or add to PATH\n$USAGE"
			exit 127
		fi
	done
}

# Check the status of any critical component, if it fails exit the script
fatal() {
	if [ "$#" -eq 2 ] && [ $1 -ne 0 ]; then
		echo "$2"
		clean
		exit 1
	fi
}

# Configuration management
newProperties() {
	CONF="$1"
}

setPrefix() {
	PREFIX="$1"
}

getProperty() {
	echo $(jq -r "$PREFIX$1" $CONF) 
}

# For Mac OS, Assumes coreutils has been installed. If not, shame on you 
if [ "$(uname)" == "Darwin" ]; then 
	echo "Mac OS detected, aliasing gtimeout"
	alias timeout="gtimeout"
fi  

