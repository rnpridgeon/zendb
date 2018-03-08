# zendb

## Description

### Installation ###

1. Install go 1.9 or greater
2. Install docker 
3. Set GOPATH env variable
  `export GOPATH=$(pwd)`
4. Get source 
  `go get github.com/rnpridgeon/zendb`

### Set-up and run ###

From your GOPATH: 

1. `./src/github.com/rnpridgeon/zendb/util/setup.sh`

2. `./bin/zendb ./src/github.com/rnpridgeon/zendb/exclude/conf.json`

### Schema Notes ###

Base tables (names in all lower case) include raw data. These are almost certainly not the tables you want to interact with directly. 

Most users will want to make use of the tables with names styled in Camel case. These join the various base tables to make the data more user-friendly. 

All base tables use Linux Epoch to store dates. These are easily converted to UTC Timestamps with `FROM_UNIXTIME(date_field)`. If you want to set this to another timezeone you can do so with `CONVERT_TZ(FROM_UNIXTIME(date_field), 'UTC', 'EST')`. To get a listing of available timezones execute the following query as the mysql root user. 

`SELECT * FROM mysql.\`time_zone_name\`;`

