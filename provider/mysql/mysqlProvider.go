package mysql

import (
	"github.com/rnpridgeon/zendb/models"
	"github.com/go-sql-driver/mysql"
	"database/sql"
	"bytes"
	"sync"
	"fmt"
	"log"
	"time"
	"strconv"
)

const (
	//TODO:move connection string to configuration so we can leverage domain sockets and TCP
	dsn = "%v:%s@tcp(%s:%d)/zendb?charset=utf8"
	sizeOf = "SELECT COUNT(1) from %s WHERE id > 0 AND updated_at >= %d;"
	fetchOrganizations = "SELECT * FROM organizations WHERE name NOT LIKE '%%deleted%%' AND id > 0 AND updated_at >= %d ORDER BY name asc;"
	fetchTickets = "SELECT * FROM %s WHERE updated_at >= %d AND status != 'deleted' ORDER BY organization_id ASC, id DESC"
	fetchByOrganizationID = "SELECT * FROM %s WHERE updated_at >= %d AND status != 'deleted' AND organization_id = %d ORDER BY id DESC"
	updateTicketMetrics = "INSERT INTO ticket_metrics(ticket_metrics.id, ticket_metrics.created_at," +
		"ticket_metrics.updated_at, ticket_metrics.ticket_id, ticket_metrics.replies, ticket_metrics.ttfr, ticket_metrics.solved_at)" +
			" SELECT ?, ?, ?, ?, ?, ?, ? FROM tickets WHERE tickets.status IN ('solved', 'closed', 'deleted') AND id=?"
)

var (
	buff    = stringBuffer{}
	inserts = make(map[string]string)
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

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("INFO: %s took %s", name, elapsed)
}

func Open(conf *MysqlConfig) *MysqlProvider {
	db, err := sql.Open(conf.Type, fmt.Sprintf(dsn,
		conf.User, conf.Password, conf.Hostname, conf.Port))

	if err != nil {
		log.Fatal("Failed to opend database: ", err)
	}

	return &MysqlProvider{db, map[string]int64{"isDirty":1}}
}

func (p *MysqlProvider) FetchState() (state map[string]int64){
	p.update()
	return p.state
}

func (p *MysqlProvider) update() {
	target := "sequence_table"

	rows, err := p.dbClient.Query(buildSelect(target, []string{"sequence_name", "last_val"}, "nil"))
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

func buildSelect(entity string, cols []string, conditional string) string {
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

	if conditional != "nil" {
		buff.WriteString(" WHERE ")
		buff.WriteString(conditional)
	}

	return buff.String()
}

func (p *MysqlProvider) CommitSequence(name string, val int64) {
	target := "sequence_table"

	stmt, _ := p.dbClient.Prepare(buildUpdate(target, []string{"last_val"}, "sequence_name"))

	_, err := stmt.Exec(val, name)
	if err != nil {
		log.Printf("SQLException: failed to insert %v into %s: \n\t%s", name, target, err)
	}
	stmt.Close()
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
	defer  timeTrack(time.Now(), "Organization Import")

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
	var err error
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, err = tx.Prepare(qry)
	} else {
		stmt, err = p.dbClient.Prepare(qry)
	}
	log.Printf("%s\n", err)
	_, err = stmt.Exec(entity.Name, entity.Created_at.Unix(), entity.Updated_at.Unix(), entity.Group_id, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s",entity.Id, target, err)
	}
}

func (p *MysqlProvider) ExportOrganizations(since int64) (entities []models.Organization) {
	defer  timeTrack(time.Now(), "Organization export")
	target := "organizations"

	var count int
	p.dbClient.QueryRow(fmt.Sprintf(sizeOf, target, since)).Scan(&count)
	entities = make([]models.Organization, count, count)

	rows, err := p.dbClient.Query(fmt.Sprintf(fetchOrganizations, since))
	defer rows.Close()

	if err != nil {
		log.Fatal("SQLException: failed to fetch from %s: %s", target, err)
	}

	var (
		raw_create int64
		raw_update int64
		index = 0
	)
	for rows.Next() {
		rows.Scan( &entities[index].Id, &entities[index].Name, &raw_create,
			&raw_update, &entities[index].Group_id)

		entities[index].Created_at = time.Unix(raw_create, 0)
		entities[index].Updated_at = time.Unix(raw_update, 0)

		index++
	}
	// Trim fat
	return entities[:index]
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportUsers(entities []models.User) {
	defer  timeTrack(time.Now(), "User import")

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
	defer  timeTrack(time.Now(), "Ticket import")

	target := "tickets"
	fields := []string{"id", "subject", "status", "requester_id", "submitter_id", "assignee_id",
		"organization_id", "group_id", "created_at", "updated_at", "version", "component", "priority", "ttfr", "solved_at"}

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
			e.Organization_id, e.Group_id, e.Created_at.Unix(), e.Updated_at.Unix(), "", "", "", 0, 0)
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

func (p *MysqlProvider) ExportTickets(since int64, orgID int64) (entities []models.Ticket_Enhanced) {
	defer  timeTrack(time.Now(), "Ticket export")
	target := "tickets"

	qry := fmt.Sprintf(fetchTickets, target, since)
	if orgID > 0 {
		qry = fmt.Sprintf(fetchByOrganizationID, target, since, orgID)
	}

	var last int
	p.dbClient.QueryRow(fmt.Sprintf(sizeOf, target, since)).Scan(&last)
	entities = make([]models.Ticket_Enhanced, last, last)
	rows, err := p.dbClient.Query(qry)
	defer rows.Close()

	if err != nil {
		log.Fatal("SQLException: failed to fetch from %s: %s", target, err)
	}

	last = 0
	var (
		raw_create int64
		raw_update int64
		raw_solved int64
		index = 0
	)

	for rows.Next() {
		rows.Scan( &entities[index].Id, &entities[index].Subject, &entities[index].Status, &entities[index].Requester_id, &entities[index].Submitter_id, &entities[index].Assignee_id,
			&entities[index].Organization_id, &entities[index].Group_id, &raw_create, &raw_update, &entities[index].Version, &entities[index].Component,
				&entities[index].Priority, &entities[index].TTFR, &raw_solved)

		entities[index].Created_at = time.Unix(raw_create, 0)
		entities[index].Updated_at = time.Unix(raw_update, 0)
		entities[index].Solved_at = time.Unix(raw_solved, 0)

		index++
	}
	return entities[:index]

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
	fields := []string{"id", "created_at", "updated_at", "ticket_id", "replies", "ttfr", "solved_at"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	stmt, _ := tx.Prepare(updateTicketMetrics)

	var last int64 = 0
	var solved int64 = 0

	for _, e := range entities {
		if solved = e.Solved_at.Unix(); solved < 0 {
			solved = 0
		}

		_, err := stmt.Exec(e.Id, e.Created_at.Unix(), e.Updated_at.Unix(), e.Ticket_id, e.Replies,e.Reply_time_in_minutes.Business, solved, e.Ticket_id)
		//TODO: Proper error handling, allow for on err callbacks
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketMetric(tx,fields, e)
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
			}
			continue
		}

		if e.Ticket_id > last && solved > 0 {
			last = e.Ticket_id
		}
	}
	stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) UpdateTicketMetric(updates []string, entity models.Ticket_metrics) {
	p.updateTicketMetric(nil, updates, entity)
}

func (p *MysqlProvider) updateTicketMetric(tx *sql.Tx, updates []string, entity models.Ticket_metrics) {
	target := "ticket_metrics"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	var solved int64
	if solved = entity.Solved_at.Unix(); solved < 0 {
		solved = 0
	}

	_, err := stmt.Exec(entity.Created_at.Unix(),entity.Updated_at.Unix(), entity.Ticket_id, entity.Replies,
		entity.Reply_time_in_minutes.Business, solved, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update id %v record in %s: \n\t%s", entity.Id, target, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportAudit(entities []models.Audit) {
	defer  timeTrack(time.Now(), "Audit import")

	target := "ticket_audit"
	fields := []string{"ticket_id", "author_id", "value"}

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	qry, found := inserts[target]
	if !found {
		qry = registerInsert(target, fields)
	}

	stmt, _ := tx.Prepare(qry)

	fieldID := strconv.FormatInt(34347708, 10)
	for _, e := range entities {
		for _, se := range e.Events {
			if se.Type == "Change" && se.Field_name == fieldID  {
				_, err := stmt.Exec(e.Ticket_id, e.Author_id, se.Value)
				if err != nil {
					switch err.(*mysql.MySQLError).Number {
					case 1062:
						p.updateAudit(tx, fields, e, se)
					default:
						log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, target, err)
				}
				continue
			}
			if e.Id > last {
				last = e.Id
			}
		}
		}
	}
		stmt.Close()

	tx.Commit()
	p.CommitSequence(target, last)
}

func (p *MysqlProvider) updateAudit(tx *sql.Tx, updates []string, entity models.Audit, sub models.Event) {
	target := "ticket_audit"

	var stmt *sql.Stmt
	qry := buildUpdate(target, updates[1:], "ticket_id")
	if tx != nil {
		stmt, _ = tx.Prepare(qry)
	} else {
		stmt, _ = p.dbClient.Prepare(qry)
	}

	_, err := stmt.Exec(entity.Author_id, sub.Value, entity.Ticket_id)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, target, err)
	}
}