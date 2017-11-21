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
 
1. Create a folder in the project directory named `./exclusive`
2. Edit `expampleConfiguration.json` with appropriate values
3. Save `exampleConfiguration.json` as `./exclusive/conf.json`
4. execute `./utils/initdb -s mysql`
5. Once start-up has completed you can run go test to populate the tables

This will obviously change in the future however it should be enough to get you started. 

Documentation will be updated as it's made useful for purposes other than future development 
