#! /usr/bin/env bash 

# Sets up databse container for sytem testing, cleans it up on EXIT
#TODO: I should probably make this accurate
USAGE="usage: ${0##*/} version"

init () {
	require jq docker
	configure
	TAG="${@: -1}"
}

configure() {
	BASE=$(realpath ./)
	SQL="$(realpath ../scripts/mysql)"
	echo $BASE
	# Read configuration file 
	newProperties ../exclude/conf.json

	USR=$(getProperty '.database.user')
	PAS=$(getProperty '.database.password')
	HOST=$(getProperty '.database.hostname')
	PORT=$(getProperty '.database.port')
}

# start(void): Starts the requested database service 
start() {
	echo "Starting $TAG service serving at $PORT ..."

	docker run --detach --name=test-mysql -p 3306:$PORT \
		-e "MYSQL_ROOT_PASSWORD=password" \
		-e "MYSQL_USER=$USR" \
		-e "MYSQL_PASSWORD=$PAS" \
		-v $SQL:/docker-entrypoint-initdb.d \
		-v $BASE:/etc/mysql/conf.d \
		$TAG --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci >/dev/null 
	fatal $? "FATAL: database container failed to start"

	# TODO: mysql readiness check should be it's own function
	local complete=1
	local timer=5
	local timeout=60

	while [ $complete -ne 0 ]; do
		echo "Initializing database..."
		if [ "$timer" -gt "$timeout" ]; then 
			fatal 1 "FATAL: database failed to start prior to timeout"
			docker logs -f test-mysql
			clean
		fi

		sleep 5;
	
		mysql -u$USR -p$PAS -h $HOSTNAME -P $PORT --protocol=tcp -e '\s' 2>/dev/null

		complete=$?
		timer=$(($timer + 5))
	done 
}

# inspect(void): Collects information necessary to construct connection string
inspect() {
	HOST=$(docker inspect test-mysql | jq -r '.[].NetworkSettings.IPAddress')
	PORT=$(docker inspect test-mysql | jq -r '.[].HostConfig.PortBindings | .[] | .[0].HostPort')
}

# test(void): Runs simple query to ensure functionality 
test() {
	echo "Running smoke test ..."
	mysql -u$USR -p$PAS -h $HOSTNAME -P $PORT -e 'use zendb; show tables' &>/dev/null
	fatal $? "Database failed sanity check"
	echo "Database start-up complete"
}

# cleanup(void): Clean up the container and it's volumes
clean() {
	if [ "$ERR" != "" ]; then
		echo -e "$ERR\n"
	fi
	echo "Cleaning up existing state..."
	docker rm -vf test-mysql 2>/dev/null
}

# TODO: wild cards get expanded which breaks the query, look into that
query() {
	configure
	local qry="$@"
	mysql -u$USR -p$PAS -h $HOSTNAME -P $PORT -e "use zendb; $qry"
}

# sanity check, both query and start require 2 configs, stop has to live with it
if [ "$#" -ne 2 ]; then 
	echo $USAGE
	exit 1
fi

# main(opts): discover self, move there so relative paths work, do work 
main() {
	self="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
	cd $self
	source utils.sh

    while getopts "sdq" opt; do
        case $opt in
            s)	init $@
				clean
                start
				inspect
				test
				echo "Access at: mysql -u$USR -p$PAS -h $HOSTNAME -P $PORT -D zendb -e 'query string'"
                exit 0;;
            d)  clean
                exit 0;;
			q)	init $@
				echo "mysql -u$USR -p$PAS -h $HOSTNAME -P $PORT -D zendb"
				exit 0;;
            \?) echo -e $USAGE
                exit 1;;
        esac
    done
    exit 1
}

main "$@"
