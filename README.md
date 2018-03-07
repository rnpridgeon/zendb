# zendb

## Description

### Installation ###

1. Install go 1.9 or greater
2. Install docker 
3. Set GOPATH env variable
  `export GOPATH=$(pwd)`
4. Get source 
  `go get github.com/rnpridgeon/zendb`
5. Build source 
  `go install github.com/rnpridgeon/zendb`

### Set-up ###

From your GOPATH: 

./src/github.com/rnpridgeon/zendb/util/setup.sh
./bin/zendb ./src/github.com/rnpridgeon/zendb/exclude/conf.json

### Schema Notes ###

Base tables (names in all lower case) include raw data. These are almost certainly not the tables you want to interact with directly. 

Most users will want to make use of the tables with names styled in Camel case. These join the various base tables to make the data more user-friendly. 

