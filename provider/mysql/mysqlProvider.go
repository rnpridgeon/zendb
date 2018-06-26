package mysql

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/rnpridgeon/structs"
	"github.com/rnpridgeon/utils"
	"github.com/rnpridgeon/utils/collections"
)

const (
	dsn_tls = "%v:%s@tcp(%s:%d)/zendb?charset=utf8&tls=skip-verify"
	dsn = "%v:%s@tcp(%s:%d)/zendb?charset=utf8"
)

type MysqlConfig struct {
	Type     string `json:"type"`
	Hostname string `json:"hostname"`
	Port     uint   `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type MysqlProvider struct {
	dbClient *sql.DB
	inserts  map[string]string
	updates  map[string]string
	preEvent map[string][]func(interface{}) interface{}
	Errors   *collections.DEQueue
}

func Open(conf *MysqlConfig) *MysqlProvider {
        if conf.Hostname == "127.0.0.1" {
	   db, err := sql.Open(conf.Type, fmt.Sprintf(dsn,
		conf.User, conf.Password, conf.Hostname, conf.Port))
        }else{
	   db, err := sql.Open(conf.Type, fmt.Sprintf(dsn_tls,
		conf.User, conf.Password, conf.Hostname, conf.Port))
        }

	err = db.Ping()

	if err != nil {
		log.Fatal("Failed to open database: ", err)
	}

	return &MysqlProvider{
		dbClient: db,
		inserts:  make(map[string]string),
		updates:  make(map[string]string),
		preEvent: make(map[string][]func(interface{}) interface{}),
		Errors:   collections.NewDEQueue(),
	}
}

func (p *MysqlProvider) Close() {
	p.dbClient.Close()
}

func (p *MysqlProvider) RegisterTransformation(target string, fn func(interface{}) interface{}) {
	p.preEvent[target] = append(p.preEvent[target], fn)
}

func (p *MysqlProvider) FetchOffset(resource string) (offset int64) {
	p.dbClient.QueryRow("select ifnull(max(updatedat),0) from " + resource).Scan(&offset)
	log.Printf("INFO: retrieved offset %d for %s\n", offset, resource)
	return offset
}

// Admittedly unsafe but necessary for the time being
func (p *MysqlProvider) ExecRaw(qry string) int64 {
	results, _ := p.dbClient.Exec(qry)
	ret, _ := results.RowsAffected()
	return ret
}

func (p *MysqlProvider) processUpdate(tx *sql.Tx, entity interface{}, name string) {

	var stmt *sql.Stmt
	var err error

	stmt, err = tx.Prepare(p.registerUpdate(entity, name))

	if err != nil {
		log.Printf("SQLException: Failed to create statement, placing batch on error queue\n %s", err)
		return
	}
	fields := append(structs.FilterValues(entity, "isKey", true), structs.FilterValues(entity, "isKey", false)...)
	_, err = stmt.Exec(fields...)

	if err != nil {
		p.Errors.Enqueue(entity)
		log.Printf("SQLException: failed to process update: \n\t%s", err)
	}

	stmt.Close()
}

func (p *MysqlProvider) processImport(entities interface{}) {
	ctx := reflect.Indirect(reflect.ValueOf(entities))

	if ctx.Len() <= 0 {
		log.Printf("WARN: No records to import, aborting operation")
		return
	}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	obj := ctx.Index(0).Interface()
	name := structs.Name(obj)

	defer utils.TimeTrack(time.Now(), fmt.Sprintf("%s import", name))

	stmt, err := tx.Prepare(p.registerInsert(obj, name))

	if err != nil {
		p.Errors.Enqueue(err)
		log.Printf("SQLException: Failed to create statement for batch %s, placing batch on error queue\n %s", ctx.Interface(), err)
		return
	}

	for i := 0; i < ctx.Len(); i++ {
		obj = ctx.Index(i).Interface()

		for _, f := range p.preEvent[name] {
			obj = f(obj)
		}

		if obj == nil {
			continue
		}

		_, err := stmt.Exec(structs.Values(obj)...)

		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.processUpdate(tx, obj, name)
				break
			default:
				p.Errors.Enqueue(obj)
				log.Printf("ERROR: failed to process %s\n\t%s", ctx.Type(), err)
			}
			continue
		}
	}

	stmt.Close()
	tx.Commit()
}

func (p *MysqlProvider) registerUpdate(i interface{}, name string) (qry string) {
	if qry, found := p.updates[name]; found {
		return qry
	}

	b := bytes.NewBufferString("UPDATE ")
	b.WriteString(strings.ToLower(name))

	fields := structs.FilterNames(i, "isKey", true)
	keys := structs.FilterNames(i, "isKey", false)

	b.WriteString(" SET " + strings.ToLower(fields[0]) + "= ?")
	for i := 1; i < len(fields); i++ {
		b.WriteString(", " + fields[i] + "= ?")
	}
	b.WriteString(" WHERE " + keys[0] + "= ? ")
	for i := 1; i < len(keys); i++ {
		b.WriteString(" AND " + keys[i] + "= ? ")
	}
	b.WriteString(";")

	log.Printf("INFO: Registering new update query with provider\n%s\n", b.Bytes())
	p.updates[name] = b.String()
	return b.String()
}

func (p *MysqlProvider) registerInsert(i interface{}, name string) (qry string) {
	if qry, ok := p.inserts[name]; ok {
		return qry
	}

	b := bytes.NewBufferString("INSERT INTO ")
	b.WriteString(strings.ToLower(name))

	fields := structs.Names(i)
	b.WriteString("(" + strings.ToLower(fields[0]))
	for i := 1; i < len(fields); i++ {
		b.WriteString(", " + fields[i])
	}
	b.WriteString(") VALUES ( ? ")

	for i := 1; i < len(fields); i++ {
		b.WriteString(", ? ")
	}
	b.WriteString(") ")

	log.Printf("INFO: Registering new Insert query with provider\n%s\n", b.Bytes())
	p.inserts[name] = b.String()

	return b.String()
}

func (p *MysqlProvider) Flush(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket Fields import")
	p.processImport(entities)
}
