package mysql

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"database/sql"

	"github.com/go-sql-driver/mysql"

	"github.com/rnpridgeon/structs"
	"github.com/rnpridgeon/utils"
	"github.com/rnpridgeon/utils/collections"
	"github.com/rnpridgeon/zendb/models"
)

const (
	ORGANIZATIONS = "organizations"
	TICKETS       = "[]Tickets"
)

const (
	//TODO:move connection string to configuration so we can leverage domain sockets and TCP
	dsn                = "%v:%s@tcp(%s:%d)/zendb?charset=utf8"
	sizeOf             = "SELECT COUNT(1) from %s WHERE id > 0 AND updatedat >= %d;"
	fetchOrganizations = "SELECT * FROM organizations WHERE name NOT LIKE '%%deleted%%' AND id > 0 AND updatedat >= %d ORDER BY name asc;"
	fetchTickets       = "SELECT * FROM tickets WHERE updatedat >= %d AND status != 'deleted' ORDER BY organizationid ASC, id DESC"
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
	db, err := sql.Open(conf.Type, fmt.Sprintf(dsn,
		conf.User, conf.Password, conf.Hostname, conf.Port))

	err = db.Ping()

	if err != nil {
		log.Fatal("Failed to opend database: ", err)
	}

	return &MysqlProvider{
		dbClient: db,
		inserts:  make(map[string]string),
		updates:  make(map[string]string),
		preEvent: make(map[string][]func(interface{}) interface{}),
		Errors:   collections.NewDEQueue(),
	}
}

func (p *MysqlProvider) RegisterTransformation(target string, fn func(interface{}) interface{}) {
	p.preEvent[target] = append(p.preEvent[target], fn)
}

func (p *MysqlProvider) FetchOffset(resource string) (offset int64) {
	p.dbClient.QueryRow("select ifnull(max(updatedat),0) from ? ", resource).Scan(&offset)
	log.Printf("INFO: retrieved offset %d for %s\n", offset, resource)
	return offset
}

// Admittedly unsafe but necessary for the time being
func (p *MysqlProvider) ExecRaw(qry string) int64 {
	results, _ := p.dbClient.Exec(qry)
	ret, _ := results.RowsAffected()
	return ret
}

func (p *MysqlProvider) ExportOrganizations(since int64) (entities []models.Organization) {
	defer utils.TimeTrack(time.Now(), "Organization export")

	var count int
	p.dbClient.QueryRow(fmt.Sprintf(sizeOf, ORGANIZATIONS, since)).Scan(&count)
	entities = make([]models.Organization, count, count)

	rows, err := p.dbClient.Query(fmt.Sprintf(fetchOrganizations, since))
	defer rows.Close()

	if err != nil {
		log.Fatal("SQLException: failed to fetch from %s: %s", ORGANIZATIONS, err)
	}

	index := 0
	for rows.Next() {
		rows.Scan(&entities[index].Id, &entities[index].Name, &entities[index].CreatedAt,
			&entities[index].UpdatedAt, &entities[index].GroupId)

		index++
	}
	// Trim fat
	return entities[:index]
}

func (p *MysqlProvider) processUpdate(tx *sql.Tx, entity interface{}) {

	var stmt *sql.Stmt
	var err error

	stmt, err = tx.Prepare(p.registerUpdate(entity))

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

	stmt, err := tx.Prepare(p.registerInsert(ctx.Index(0).Interface()))

	if err != nil {
		p.Errors.Enqueue(err)
		log.Printf("SQLException: Failed to create statement for batch %s, placing batch on error queue\n %s", ctx.Interface(), err)
		return
	}

	for i := 0; i < ctx.Len(); i++ {
		obj := ctx.Index(i).Interface()

		for _, f := range p.preEvent[structs.Name(obj)] {
			obj = f(obj)
		}

		if obj == nil {
			continue
		}

		_, err := stmt.Exec(structs.Values(obj)...)

		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.processUpdate(tx, obj)
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

func (p *MysqlProvider) registerUpdate(i interface{}) (qry string) {
	if qry, found := p.updates[structs.Name(i)]; found {
		return qry
	}

	b := bytes.NewBufferString("UPDATE ")
	b.WriteString(strings.ToLower(structs.Name(i)))

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
	p.updates[structs.Name(i)] = b.String()
	return b.String()
}

func (p *MysqlProvider) registerInsert(i interface{}) (qry string) {
	if qry, ok := p.inserts[structs.Name(i)]; ok {
		return qry
	}

	b := bytes.NewBufferString("INSERT INTO ")
	b.WriteString(strings.ToLower(structs.Name(i)))

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
	p.inserts[structs.Name(i)] = b.String()

	return b.String()
}

func (p *MysqlProvider) ImportTicketFields(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket Fields import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportGroups(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Group import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportOrganizations(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Organization import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportOrganizationFields(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Organization Fields import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportUsers(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "User import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportUserFields(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "User Fields import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportTickets(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportTicketMetrics(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket Metrics import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportAudit(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket Audit import")
	p.processImport(entities)
}

func (p *MysqlProvider) ImportTicketCustomFields(entities interface{}) {
	defer utils.TimeTrack(time.Now(), "Ticket Custom Field import")
	p.processImport(entities)
}

func (p *MysqlProvider) ExportTickets(since int64, orgID int64) (entities []models.TicketEnhanced) {
	defer utils.TimeTrack(time.Now(), "Ticket export")

	qry := fmt.Sprintf(fetchTickets, since)

	var last int
	p.dbClient.QueryRow(fmt.Sprintf(sizeOf, TICKETS, since)).Scan(&last)
	entities = make([]models.TicketEnhanced, last, last)
	rows, err := p.dbClient.Query(qry)
	defer rows.Close()

	if err != nil {
		log.Fatal("SQLException: failed to fetch from %s: %s", TICKETS, err)
	}

	last = 0
	index := 0
	for rows.Next() {
		rows.Scan(&entities[index].Id, &entities[index].Subject, &entities[index].Status, &entities[index].RequesterId, &entities[index].SubmitterId, &entities[index].AssigneeId,
			&entities[index].OrganizationId, &entities[index].GroupId, entities[index].CreatedAt, entities[index].UpdatedAt, &entities[index].Version, &entities[index].Component,
			&entities[index].Priority, &entities[index].TTFR, entities[index].Solved_at)

		index++
	}
	return entities[:index]

}
