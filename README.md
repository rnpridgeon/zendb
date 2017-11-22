# zendb

## Description

### !!Work in progress!! ###

Simple tool which extract data from Zendesk and places it into a relational database, mysql in this case. This makes it easier to extend upon the platform with custom tooling to make workflows and reporting easier. 

There is not currently any documentation available as I am still fleshing out the details and retooling things where necessary. There is however a handy script under /utils which will set up 

# Dependencies 

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
 
Assuming you have all the depenencies in place and admin rights within zendesk execute the following from the project's root directory. 

`./util/setup.sh` 

Answer some questions, wait. There is some junk in ZD from tickets/users being deleted and moved around. This causes a few key constraint errors to be thrown; this is true of for bulk imports as well. Zendesk's export functinality includes all assets which have changed since the start-date, as a result you get both new and updated object. This will be cleaned up in later. 

