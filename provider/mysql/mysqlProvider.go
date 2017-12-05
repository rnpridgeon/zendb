package mysql

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/rnpridgeon/zendb/models"
	"log"
	"strconv"
	"time"
)

// TODO: create a simple stats library for ingesting/storing time tracking data
func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("INFO: %s took %s", name, elapsed)
}

// targets
const (
	SEQUENCE_TABLE = "sequence_table"

	TICKET_FIELDS       = "ticket_fields"
	TICKET_FIELD_VALUES = "ticket_metadata"

	GROUPS        = "groups"
	ORGANIZATIONS = "organizations"
	USERS         = "users"
	TICKETS       = "tickets"

	TICKET_METRICS = "ticket_metrics"
	TICKET_AUDITS  = "ticket_audits"
)

const (
	//TODO:move connection string to configuration so we can leverage domain sockets and TCP
	dsn    = "%v:%s@tcp(%s:%d)/zendb?charset=utf8"
	sizeOf = "SELECT COUNT(1) from %s WHERE id > 0 AND updated_at >= %d;"

	// Progress tracking
	importSequence = " INSERT INTO " + SEQUENCE_TABLE + "(sequence_name, last_val) VALUES (?, ?)"
	updateSequence = " UPDATE " + SEQUENCE_TABLE + " SET last_val = ? WHERE sequence_name = ?"
	fetchSequence  = " SELECT sequence_name, last_val from " + SEQUENCE_TABLE + ";"

	// Metadata
	importTicketFields      = "INSERT INTO " + TICKET_FIELDS + "(id, title) VALUES (?, ?);"
	importTicketFieldValues = "INSERT INTO " + TICKET_FIELD_VALUES + "(ticket_id, field_id, raw_value, transformed_value)" +
		"VALUES(?, ?, ?, ?);"
	updateTicketFields      = "UPDATE " + TICKET_FIELDS + " SET title = ? WHERE id = ?"
	updateTicketFieldValues = "UPDATE " + TICKET_FIELD_VALUES + " SET raw_value = ? , transformed_value = ? WHERE ticket_id = ? AND field_id = ?"

	// Main resources
	importGroups        = "INSERT INTO " + GROUPS + "(id, name, created_at, updated_at) VALUES(?, ?, ?, ?);"
	importOrganizations = "INSERT INTO " + ORGANIZATIONS + "(id, name, created_at, updated_at, group_id) VALUES(?, ?, ?, ?, ?);"
	importUsers         = "INSERT INTO " + USERS + "(id, email, name, created_at, organization_id, default_group_id, role, time_zone, " +
		"updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);"
	importTickets = "INSERT INTO " + TICKETS + "(id, subject, status, requester_id, submitter_id, assignee_id, " +
		"organization_id , group_id, created_at, updated_at, cause, version, component, priority, ttfr, solved_at) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);"
	importTicketMetrics = "INSERT INTO " + TICKET_METRICS + "(id, created_at, updated_at, ticket_id, replies, ttfr, solved_at) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?);"
	importTicketAudits = "INSERT INTO " + TICKET_AUDITS + "(ticket_id, author_id, value) VALUES(?, ?, ?);"

	// Update main resources
	updateGroups        = "UPDATE " + GROUPS + " SET name =?, created_at= ?, updated_at= ? WHERE id= ?;"
	updateOrganizations = "UPDATE " + ORGANIZATIONS + " SET name= ?, created_at= ?, updated_at= ?, group_id= ? WHERE id = ?;"
	updateUsers         = "UPDATE " + USERS + " SET email= ?, name= ?, created_at= ?, organization_id= ?, default_group_id= ?, " +
		"role= ?, time_zone= ?,updated_at= ? WHERE id =?;"
	updateTickets = "UPDATE " + TICKETS + " SET subject= ?, status= ?, requester_id= ?, submitter_id= ?, assignee_id= ?, " +
		"organization_id= ?, group_id= ?, created_at= ?, updated_at= ? WHERE id = ?;"
	updateTicketMetrics = "UPDATE " + TICKET_METRICS + " SET created_at= ?, updated_at= ?, ticket_id= ?, replies= ?, " +
		"ttfr= ?, solved_at= ? WHERE id =?;"
	updateTicketAudits = "UPDATE " + TICKET_AUDITS + " SET author_id= ?, value= ? WHERE ticket_id = ?;"

	fetchGroups        = ""
	fetchOrganizations = "SELECT * FROM organizations WHERE name NOT LIKE '%%deleted%%' AND id > 0 AND updated_at >= %d ORDER BY name asc;"
	fetchUsers         = ""
	fetchTickets       = "SELECT * FROM tickets WHERE updated_at >= %d AND status != 'deleted' ORDER BY organization_id ASC, id DESC"
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
	state    map[string]int64
	preEvent map[string][]func(interface{})
}

func Open(conf *MysqlConfig) *MysqlProvider {
	db, err := sql.Open(conf.Type, fmt.Sprintf(dsn,
		conf.User, conf.Password, conf.Hostname, conf.Port))

	if err != nil {
		log.Fatal("Failed to opend database: ", err)
	}

	return &MysqlProvider{
		db,
		map[string]int64{"isDirty": 1},
		make(map[string][]func(interface{}))}
}

func (p *MysqlProvider) RegisterTransformation(target string, fn func(interface{})) {
	p.preEvent[target] = append(p.preEvent[target], fn)
}

func (p *MysqlProvider) FetchState() (state map[string]int64) {
	p.update()
	return p.state
}

func (p *MysqlProvider) update() {
	rows, err := p.dbClient.Query(fetchSequence)
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

func (p *MysqlProvider) CommitSequence(name string, val int64) {
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	p.commitSequence(tx, name, val)

	tx.Commit()
}

func (p *MysqlProvider) commitSequence(tx *sql.Tx, name string, val int64) {
	stmt, _ := tx.Prepare(importSequence)
	_, err := stmt.Exec(name, val)
	if err != nil {
		switch err.(*mysql.MySQLError).Number {
		case 1062:
			p.updateSequence(tx, name, val)
			break
		default:
			log.Printf("SQLException: failed to insert %v into %s: \n\t%s", name, SEQUENCE_TABLE, err)
		}
	}
	stmt.Close()
}

func (p *MysqlProvider) updateSequence(tx *sql.Tx, name string, val int64) {
	stmt, _ := tx.Prepare(updateSequence)

	_, err := stmt.Exec(val, name)
	if err != nil {
		log.Printf("SQLException: failed to insert %v into %s: \n\t%s", name, SEQUENCE_TABLE, err)
	}

	stmt.Close()
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportGroups(entities []models.Group) {
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0
	stmt, _ := tx.Prepare(importGroups)
	for _, e := range entities {

		for _, f := range p.preEvent[GROUPS] {
			f(&e)
		}

		_, err := stmt.Exec(e.Id, e.Name, e.Created_at, e.Updated_at)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateGroup(tx, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, GROUPS, err)
			}
			continue
		}

		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, GROUPS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateGroup(updates []string, entity models.Group) {
	p.updateGroup(nil, entity)
}

func (p *MysqlProvider) updateGroup(tx *sql.Tx, entity models.Group) {
	var stmt *sql.Stmt

	if tx != nil {
		stmt, _ = tx.Prepare(updateGroups)
	} else {
		stmt, _ = p.dbClient.Prepare(updateGroups)
	}

	_, err := stmt.Exec(entity.Name, entity.Created_at, entity.Updated_at, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, GROUPS, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportOrganizations(entities []models.Organization) {
	defer timeTrack(time.Now(), "Organization Import")

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importOrganizations)
	for _, e := range entities {

		for _, f := range p.preEvent[ORGANIZATIONS] {
			f(&e)
		}

		_, err := stmt.Exec(e.Id, e.Name, e.Created_at, e.Updated_at, e.Group_id)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateOrganization(tx, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, ORGANIZATIONS, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, ORGANIZATIONS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateOrganization(entity models.Organization) {
	p.updateOrganization(nil, entity)
}

func (p *MysqlProvider) updateOrganization(tx *sql.Tx, entity models.Organization) {
	var stmt *sql.Stmt

	if tx != nil {
		stmt, _ = tx.Prepare(updateOrganizations)
	} else {
		stmt, _ = p.dbClient.Prepare(updateOrganizations)
	}

	_, err := stmt.Exec(entity.Name, entity.Created_at, entity.Updated_at, entity.Group_id, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, ORGANIZATIONS, err)
	}
}

func (p *MysqlProvider) ExportOrganizations(since int64) (entities []models.Organization) {
	defer timeTrack(time.Now(), "Organization export")

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
		rows.Scan(&entities[index].Id, &entities[index].Name, &entities[index].Created_at,
			&entities[index].Updated_at, &entities[index].Group_id)

		index++
	}
	// Trim fat
	return entities[:index]
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportUsers(entities []models.User) {
	defer timeTrack(time.Now(), "User import")

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importUsers)
	for _, e := range entities {

		for _, f := range p.preEvent[USERS] {
			f(&e)
		}

		_, err := stmt.Exec(e.Id, e.Email, e.Name, e.Created_at, e.Organization_id,
			e.Default_group_id, e.Role, e.Time_zone, e.Updated_at)

		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateUser(tx, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, USERS, err)
			}
			continue
		}

		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, USERS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateUser(entity models.User) {
	p.updateUser(nil, entity)
}

func (p *MysqlProvider) updateUser(tx *sql.Tx, entity models.User) {

	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateUsers)
	} else {
		stmt, _ = p.dbClient.Prepare(updateUsers)
	}

	_, err := stmt.Exec(entity.Email, entity.Name, entity.Created_at, entity.Organization_id,
		entity.Default_group_id, entity.Role, entity.Time_zone, entity.Updated_at, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, USERS, err)
	}
}

func (p *MysqlProvider) ImportTickets(entities []models.Ticket) {
	defer timeTrack(time.Now(), "Ticket import")

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importTickets)

	for _, e := range entities {

		for _, f := range p.preEvent[TICKETS] {
			f(&e)
		}

		_, err := stmt.Exec(e.Id, e.Subject, e.Status, e.Requester_id, e.Submitter_id, e.Assignee_id,
			e.Organization_id, e.Group_id, e.Created_at, e.Updated_at, "", "", "", "", 0, 0)

		p.ImportTicketFieldValues(e.Id, e.Custom_fields)

		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicket(tx, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, TICKETS, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, TICKETS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateTicket(entity models.Ticket) {
	p.updateTicket(nil, entity)
}

func (p *MysqlProvider) updateTicket(tx *sql.Tx, entity models.Ticket) {

	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateTickets)
	} else {
		stmt, _ = p.dbClient.Prepare(updateTickets)
	}

	_, err := stmt.Exec(entity.Subject, entity.Status, entity.Requester_id, entity.Submitter_id, entity.Assignee_id,
		entity.Organization_id, entity.Group_id, entity.Created_at, entity.Updated_at, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v in %s: \n\t%s", entity.Id, TICKETS, err)
	}
}

func (p *MysqlProvider) ExportTickets(since int64, orgID int64) (entities []models.Ticket_Enhanced) {
	defer timeTrack(time.Now(), "Ticket export")

	qry := fmt.Sprintf(fetchTickets, since)

	var last int
	p.dbClient.QueryRow(fmt.Sprintf(sizeOf, TICKETS, since)).Scan(&last)
	entities = make([]models.Ticket_Enhanced, last, last)
	rows, err := p.dbClient.Query(qry)
	defer rows.Close()

	if err != nil {
		log.Fatal("SQLException: failed to fetch from %s: %s", TICKETS, err)
	}

	last = 0
	index := 0
	for rows.Next() {
		rows.Scan(&entities[index].Id, &entities[index].Subject, &entities[index].Status, &entities[index].Requester_id, &entities[index].Submitter_id, &entities[index].Assignee_id,
			&entities[index].Organization_id, &entities[index].Group_id, entities[index].Created_at, entities[index].Updated_at, &entities[index].Version, &entities[index].Component,
			&entities[index].Priority, &entities[index].TTFR, entities[index].Solved_at)

		index++
	}
	return entities[:index]

}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTicketFields(entities []models.Ticket_field) {
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importTicketFields)
	for _, e := range entities {

		for _, f := range p.preEvent[TICKET_FIELDS] {
			f(&e)
		}

		_, err := stmt.Exec(e.Id, e.Title)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketField(tx, e)
				break
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, TICKET_FIELDS, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, TICKETS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateTicketField(entity models.Ticket_field) {
	p.updateTicketField(nil, entity)
}

func (p *MysqlProvider) updateTicketField(tx *sql.Tx, entity models.Ticket_field) {
	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateTicketFields)
	} else {
		stmt, _ = p.dbClient.Prepare(updateTicketFields)
	}

	_, err := stmt.Exec(entity.Title, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, TICKET_FIELDS, err)
	}
}

func (p *MysqlProvider) ImportTicketFieldValues(parent int64, entities []models.Custom_fields) {
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importTicketFieldValues)

	for _, e := range entities {

		for _, f := range p.preEvent[TICKET_FIELD_VALUES] {
			f(&e)
		}
		_, err := stmt.Exec(parent, e.Id, e.Value, e.Transformed)
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketFieldValues(tx, parent, e)
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, TICKET_FIELD_VALUES, err)
			}
			continue
		}
		if e.Id > last {
			last = e.Id
		}
	}
	stmt.Close()
	p.commitSequence(tx, TICKET_FIELD_VALUES, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateTicketFieldValues(parent int64, entity models.Custom_fields) {
	p.updateTicketFieldValues(nil, parent, entity)
}

func (p *MysqlProvider) updateTicketFieldValues(tx *sql.Tx, parent int64, entity models.Custom_fields) {
	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateTicketFieldValues)
	} else {
		stmt, _ = p.dbClient.Prepare(updateTicketFieldValues)
	}

	_, err := stmt.Exec(entity.Value, entity.Transformed, entity.Id, parent)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, TICKET_FIELD_VALUES, err)
	}
}

//TODO: Reduce code redundancy
func (p *MysqlProvider) ImportTicketMetrics(entities []models.Ticket_metrics) {
	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	stmt, _ := tx.Prepare(importTicketMetrics)

	var last int64 = 0
	for _, e := range entities {

		for _, f := range p.preEvent[TICKET_METRICS] {
			f(&e)
		}
		_, err := stmt.Exec(e.Id, e.Created_at, e.Updated_at, e.Ticket_id, e.Replies,
			e.Reply_time_in_minutes.Business, e.Solved_at)
		//TODO: Proper error handling, allow for on err callbacks
		if err != nil {
			switch err.(*mysql.MySQLError).Number {
			case 1062:
				p.updateTicketMetric(tx, e)
			default:
				log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, TICKET_METRICS, err)
			}
			continue
		}
		// TODO: this actually short changes us
		if e.Ticket_id > last && e.Solved_at > 0 {
			last = e.Ticket_id
		}
	}
	stmt.Close()
	p.commitSequence(tx, TICKET_METRICS, last)
	tx.Commit()
}

func (p *MysqlProvider) UpdateTicketMetric(entity models.Ticket_metrics) {
	p.updateTicketMetric(nil, entity)
}

func (p *MysqlProvider) updateTicketMetric(tx *sql.Tx, entity models.Ticket_metrics) {

	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateTicketMetrics)
	} else {
		stmt, _ = p.dbClient.Prepare(updateTicketMetrics)
	}

	_, err := stmt.Exec(entity.Created_at, entity.Updated_at, entity.Ticket_id, entity.Replies,
		entity.Reply_time_in_minutes.Business, entity.Solved_at, entity.Id)

	if err != nil {
		log.Printf("SQLException: failed to update id %v record in %s: \n\t%s", entity.Id, TICKET_METRICS, err)
	}
}

func (p *MysqlProvider) ImportAudit(entities []models.Audit) {
	defer timeTrack(time.Now(), "Audit import")

	tx, _ := p.dbClient.Begin()
	defer tx.Rollback()

	var last int64 = 0

	stmt, _ := tx.Prepare(importTicketAudits)

	fieldID := strconv.FormatInt(34347708, 10)
	for _, e := range entities {

		for _, f := range p.preEvent[TICKET_AUDITS] {
			f(&e)
		}
		for _, se := range e.Events {
			if se.Type == "Change" && se.Field_name == fieldID {
				_, err := stmt.Exec(e.Ticket_id, e.Author_id, se.Value)
				if err != nil {
					switch err.(*mysql.MySQLError).Number {
					case 1062:
						p.updateAudit(tx, e, se)
					default:
						log.Printf("SQLException: failed to insert %v into %s: \n\t%s", e.Id, TICKET_AUDITS, err)
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
	p.commitSequence(tx, TICKET_AUDITS, last)
	tx.Commit()
}

func (p *MysqlProvider) updateAudit(tx *sql.Tx, entity models.Audit, sub models.Event) {
	defer timeTrack(time.Now(), "Audit update")

	var stmt *sql.Stmt
	if tx != nil {
		stmt, _ = tx.Prepare(updateTicketAudits)
	} else {
		stmt, _ = p.dbClient.Prepare(updateTicketAudits)
	}
	_, err := stmt.Exec(entity.Author_id, sub.Value, entity.Ticket_id)

	if err != nil {
		log.Printf("SQLException: failed to update %v record in %s: \n\t%s", entity.Id, TICKET_AUDITS, err)
	}
}
