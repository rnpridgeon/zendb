# zendb

## Description

### !!Work in progress!! ###

Simple which extracts from data Zendesk and places it into a relational database; mysql in this case. When complete this will provide the means for extending Zendesk to enhance workflows. This also simplifies reporting a bit as I am not incredibly fond of the existing tooling. 

Documentation to follow project completion, in the meantime driver_test.go touches everyhing. 

# Dependencies ( Assumes Mac OS, no low-level libraries were used so it should be fairly portable) 

-Go 1.9 *This is the only version it has been tested on. 
  `brew install go`

-mysql driver: 
  `go get -u github.com/go-sql-driver/mysql`

-mysql: 
  `brew install mysql`

There is a utility script for using mysql docker as well 
Docker:
  https://docs.docker.com/docker-for-mac/install/

# Set-up (Assumes use of docker)

1. Set GOPATH

`export GOPATH=/some/go/dir`

2. Fetch the project and dependencies with go get 

`go get github.com/go-sql-driver/mysql`
`go get github.com/rnpridgeon/zendb`

3. Assuming you have all the depenencies in place and admin rights within zendesk execute the following from the project's root directory. 

`./util/setup.sh` 

Answer some questions, wait. Once populated you can execute `/util/initdb.sh -q mysql` for an example of how to connect using the mysql client. 
