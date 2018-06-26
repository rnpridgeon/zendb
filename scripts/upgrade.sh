CONF=/etc/zendb/zendb.json

if [ "$#" -eq 1 ]; then
        CONF=$1
fi

git -C $GOPATH/src/github.com/rnpridgeon/zendb pull
go install github.com/rnpridgeon/zendb
sed -i s/'password'/$(cat $CONF | jq .database.password | sed s/\"//g)/ $GOPATH/src/github.com/rnpridgeon/zendb/scripts/mysql/mysql.sql
