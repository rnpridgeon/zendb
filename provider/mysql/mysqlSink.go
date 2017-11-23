package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/rnpridgeon/zendb/models"
	"database/sql"
	"bytes"
	"sync"
	"fmt"
	"log"
	"github.com/go-sql-driver/mysql"
)

//TODO: Template out this entire file so make database support `plugable`,
const (
	dsn = "%v:%s@tcp(%s:%d)/zendb?charset=utf8"
)

var (
	buff    = stringBuffer{}
	inserts = make(map[string]string)
	selects = make(map[string]string)
)

type stringBuffer struct {
	bytes.Buffer
	sync.Mutex
}

type MysqlProvider struct {
	dbClient *sql.DB
	state    map[string]int64
}

type MysqlConfig struct {
	Type     string `json:"type"`
	Hostname string `json:"hostname"`
	Port     uint   `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func Open(conf *MysqlConfig) *MysqlProvider {
	db, _ := sql.Open(conf.Type, fmt.Sprintf(dsn,
		conf.User, conf.Password, conf.Hostname, conf.Port))
	return &MysqlProvider{db, map[string]int64{"isDirty":1}}
}

func (p *MysqlProvider) FetchState() (state map[string]int64){
	if p.state["isDirty"] != 0 {
		p.update()
		p.state["isDirty"] = 0
	}

	return p.state
}

func dirty(s map[string]int64) {
	s["isDirty"] = 1
}

func (p *MysqlProvider) update() {
	target := "sequence_table"

	qry, found := selects[target]
	if !found {
		qry = registerSelect(target, []string{"sequence_name", "last_val"}, "nil")
	}

	rows, err := p.dbClient.Query(qry)
	defer rows.Close()

	if err != nil {
		log.Println(err)
	}

	var k string
	var v int64
	for rows.Next() {
		rows.Scan(&k, &v)
		p.state[k] = v
	}

}

// Admittedly unsafe but necessary for the time being
func (p *MysqlProvider) ExecRaw(qry string) int64 {
	results, _ := p.dbClient.Exec(qry)
	ret, _ := results.RowsAffected()
	return ret
}

// Build prepared query string, TODO: templates are probably a smarter way to handle this
func registerInsert(entity string, cols []string) string {
	buff.Lock()
	defer buff.Unlock()

	buff.Reset()

	buff.WriteString(" INSERT INTO ")
	buff.WriteString(entity)
	buff.WriteString(" VALUES ( ?")

	for i := 1; i < len(cols); i++ {
		buff.WriteString(", ?")
	}

	buff.WriteString(");")

	inserts[entity] = buff.String()

	return inserts[entity]
}

// Build prepared query string, TODO: this can be worked into registerImport
func buildUpdate(entity string, cols []string, conditional string) string {
	buff.Lock()
	defer buff.Unlock()

	buff.Reset()

	buff.WriteString(" UPDATE ")
	buff.WriteString(entity)
	buff.WriteString(" SET ")
	buff.WriteString(cols[0])
	buff.WriteString(" = ? ")

	for i := 1; i < len(cols); i++ {
		buff.WriteRune(',')
		buff.WriteString(cols[i])
		buff.WriteString("= ? ")
	}

	buff.WriteString(" WHERE ")
	buff.WriteString(conditional)
	buff.WriteString("= ?;")

	return buff.String()
}

func registerSelect(entity string, cols []string, conditional string) string {
	buff.Lock()
	defer buff.Unlock()

	buff.Reset()

	buff.WriteString(" SELECT ")
	buff.WriteString(cols[0])

	for i := 1; i < len(cols); i++ {
		buff.WriteRune(',')
		buff.WriteString(cols[i])
	}
	buff.WriteString(" FROM ")
	buff.WriteString(entity)
	// TODO: this is terrible, fix it
	if conditional != "nil" {
		buff.WriteString(" WHERE ")
		buff.WriteString(conditional)
		buff.WriteString("= ?;")
	}
	selects[entity] = buff.String()

	return selects[entity]
}

func (p *MysqlProvider) CommitSequence(name string, val int64) {
	target := "sequence_table"

	stmt, _ := p.dbClient.Prepare(buildUpdate(target, []string{"last_val"}, "sequence_name"))

	_, err := stmt.Exec(val, name)
	if err != nil {
		log.Printf("SQLException: failed to insert %v into %s: \n\t%s", name, target, err)
	}
	stmt.Close()
	dirty(p.state)
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportGroups(entities []models.Group) {
	target := "groups"
	fields := []string{"id", "name", "created_at", "updated_at"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(e.Id, e.Name, e.Created_at.Unix(), e.Updated_at.Unix())
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateGroup(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}

	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateGroup(updates []string, entity models.Group) {
	p.updateGroup(nil, updates, entity)
}

func (p *MysqlProvider) updateGroup(tx *sql.Tx, updates []string, entity models.Group) {
	target := "groups"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}


	_, err := stmt.Exec(entity.Name, entity.Created_at.Unix(), entity.Updated_at.Unix(), entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportOrganizations(entities []models.Organization) {
	target := "organizations"
	fields := []string{"id", "name", "created_at", "updated_at", "group_id"}
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(e.Id, e.Name, e.Created_at.Unix(), e.Updated_at.Unix(), e.Group_id)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateOrganization(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}

	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateOrganization(updates []string, entity models.Organization) {
	p.updateOrganization(nil, updates, entity)
}

func (p *MysqlProvider) updateOrganization(tx *sql.Tx, updates []string, entity models.Organization) {
	target := "organizations"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(entity.Name, entity.Created_at.Unix(), entity.Updated_at.Unix(), entity.Group_id, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s",entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportUsers(entities []models.User) {
	target := "users"
	fields := []string{"id", "email", "name", "created_at", "organization_id",
		"default_group_id", "role", "time_zone", "updated_at"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(e.Id, e.Email, e.Name, e.Created_at.Unix(), e.Organization_id,
			e.Default_group_id, e.Role, e.Time_zone, e.Updated_at.Unix())
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateUser(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateUser(updates []string, entity models.User) {
	p.updateUser(nil, updates, entity)
}

func (p *MysqlProvider) updateUser(tx *sql.Tx, updates []string, entity models.User) {
	target := "users"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(entity.Email, entity.Name, entity.Created_at.Unix(), entity.Organization_id,
		entity.Default_group_id, entity.Role, entity.Time_zone, entity.Updated_at.Unix(), entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTickets(entities []models.Ticket) {
	target := "tickets"
	fields := []string{"id", "subject", "status", "requester_id", "submitter_id", "assignee_id",
		"organization_id", "group_id", "created_at", "updated_at", "version", "component", "priority", "solved_at"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(e.Id, e.Subject, e.Status, e.Requester_id, e.Submitter_id, e.Assignee_id,
			e.Organization_id, e.Group_id, e.Created_at.Unix(), e.Updated_at.Unix(), "", "", "", 0)
		p.ImportTicketCustomFields(e.Id, e.Custom_fields)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicket(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateTicket(updates []string, entity models.Ticket) {
	p.updateTicket(nil, updates, entity)
}

func (p *MysqlProvider) updateTicket(tx *sql.Tx, updates []string, entity models.Ticket) {
	target := "tickets"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:10], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(entity.Subject, entity.Status, entity.Requester_id, entity.Submitter_id, entity.Assignee_id,
		entity.Organization_id, entity.Group_id, entity.Created_at.Unix(), entity.Updated_at.Unix(), entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTicketFields(entities []models.Ticket_field) {
	target := "ticket_fields"
	fields := []string{"id", "title"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(e.Id, e.Title)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketField(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateTicketField(updates []string, entity models.Ticket_field) {
	p.updateTicketField(nil, updates, entity)
}

func (p *MysqlProvider) updateTicketField(tx *sql.Tx, updates []string, entity models.Ticket_field) {
	target := "ticket_fields"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(entity.Id, entity.Title)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTicketCustomFields(parent int64, entities []models.Custom_fields) {
	target := "ticket_metadata"
	fields := []string{"ticket_id", "field_id", "value"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)
	for _, e := range entities {
		_, err := stmt.Exec(parent, e.Id, e.Value)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketCustomField(tx,fields,parent, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateTicketCustomField(updates []string, parent int64, entity models.Custom_fields) {
	p.updateTicketCustomField(nil, updates, parent,  entity)
}

func (p *MysqlProvider) updateTicketCustomField(tx *sql.Tx, updates []string, parent int64, entity models.Custom_fields) {
	target := "ticket_metadata"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[2:], "ticket_id = ? AND field_id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec( entity.Value, entity.Id, parent)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTicketMetrics(entities []models.Ticket_metrics) {
	target := "ticket_metrics"
	fields := []string{"id", "created_at", "updated_at", "ticket_id", "replies", "solved_at"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)

	// TODO: handle nil time object at deserialization, make validation method
	var solved, created, updated int64
	for _, e := range entities {
		solved = 0
		created = 0
		updated = 0
		if e.Solved_at != nil {
			solved = e.Solved_at.Unix()
		}
		if e.Created_at != nil {
			created = e.Created_at.Unix()
		}
		if e.Updated_at != nil {
			updated = e.Updated_at.Unix()
		}

		_, err := stmt.Exec(e.Id, created, updated, e.Ticket_id, e.Replies, solved)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketMetric(tx,fields, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateTicketMetric(updates []string, entity models.Ticket_metrics) {
	p.updateTicketMetric(nil, updates, entity)
}

func (p *MysqlProvider) updateTicketMetric(tx *sql.Tx, updates []string, entity models.Ticket_metrics) {
	target := "ticket_metrics"
	// TODO: handle nil time object at deserialization, make validation method
	var solved, created, updated int64
	solved = 0
	created = 0
	updated = 0
	if entity.Solved_at != nil {
		solved = entity.Solved_at.Unix()
	}
	if entity.Created_at != nil {
		created = entity.Created_at.Unix()
	}
	if entity.Updated_at != nil {
		updated = entity.Updated_at.Unix()
	}

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(created, updated, entity.Ticket_id, entity.Replies, solved, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update id %v record in %s: \n\t%s", entity.Id, target, err)
	}
}
