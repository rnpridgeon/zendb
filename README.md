# zendb

## Description

### Installation ###

* Install Bash Coreutils ( on mac OS )
  ** `brew install coreutils`
* Install jq ( on mac OS )
  ** `brew install jq`
* Install go 1.9 or greater
* Install docker 
* Set GOPATH env variable
  ** `export GOPATH=$(pwd)`
* Get source 
  ** `go get github.com/rnpridgeon/zendb`

### Set-up and run ###

From your GOPATH: 

1. `./src/github.com/rnpridgeon/zendb/util/setup.sh`

2. `./bin/zendb ./src/github.com/rnpridgeon/zendb/exclude/conf.json`

### Usage Notes ###

#### Configuration ####
An API token needs to be used to access zendesk

 "zendesk": {
    "subdomain": "confluent",
    "user": "ssadmin@confluent.io/token",
    "password": "<api-key>"
  },

Also note that mysql version 5.7.22 is required

#### Base Tables ####
These tables are styled in all lower case characters. Thse tables are comprised of raw data as it was extracted from Zendesk. 
They are intended to be used in the creation of VIEWS which are more user-friendly and or meaningful for the typical user. 

All Zendesk ticket fields are extracted and placed into these base tables.  If the ticket fields change, the base tables will automatically
be updated to include the new fields, however changes will be required to the table views.

Only "Priority" and "Total time spent (sec)" audit entries are currently captured.  To add another field, look to extractChangeEvents.

#### Table Views ####

These logical tables have been stylized in [ Pascal case ]( http://wiki.c2.com/?PascalCase ). 

These are comprised of one or more base tables and are intended to make zendesk data more accessible. 

Most users will want to make use of these tables as opposed to the base tables described above. 

If a new field is added to zendesk, or you find that the pre-prepared tables do not answer all of your needs its recommend that you edit and or create additional views using the syntax below: 

`CREATE OR REPLACE VIEW AS [table definition]`

The pre-prepared views found at the bottom of [ mysql.sql]( ./scripts/mysql.sql ) can be used as examples for such statements. 

#### All base tables use Linux Epoch to store dates #### 

  To convert these to UTC Timestamps wrap the date type fields with the function `FROM_UNIXTIME`: 

    `SELECT FROM_UNIXTIME(createdat) FROM tickets`

  To convert UTC timestamp to a timezone specific variant use the function `CONVERT_TZ`: 

    `SELECT CONVERT_TZ(FROM_UNIXTIME(updatedat), 'UTC', 'EST') FROM tickets;`

  To get a listing of available timezones execute the following query as the mysql root user. 

    `SELECT * FROM mysql.'time_zone_name';`

