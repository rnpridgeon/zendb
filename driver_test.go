package zendb

import (
	"encoding/json"
	"github.com/rnpridgeon/zendb/provider/mysql"
	"github.com/rnpridgeon/zendb/provider/zendesk"
	"log"
	"net/http"
	"os"
	"testing"
	"fmt"
)


// TODO: make provider interface
var (
	conf Config
	sink *mysql.MysqlProvider
	source *zendesk.ZDProvider
)

type Config struct {
	ZDconf *zendesk.ZendeskConfig `json:"zendesk"`
	DBconf *mysql.MysqlConfig     `json:"database"`
}

const (
	insertPriority =`
	UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.priority = SUBSTRING(ticket_metadata.value,1,2)
		WHERE ticket_fields.title = "Case Priority"`

	insertComponent =`
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.component = ticket_metadata.value
		WHERE ticket_fields.title = "Component"`

	insertVersion =`
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.version = ticket_metadata.value
		WHERE ticket_fields.title like "%Kafka Version"`

	insertSolved = `
		UPDATE tickets
			JOIN ticket_metrics on tickets.id = ticket_metrics.ticket_id
		SET tickets.solved_at = ticket_metrics.solved_at`
	)
func successOnPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Errorf("Function failed to Panic")
	}
}

// Test custom query/post processing
func testRaw() {
	sink.ExecRaw(insertPriority)
	sink.ExecRaw(insertComponent)
	sink.ExecRaw(insertVersion)
	sink.ExecRaw(insertSolved)
}

func testDataLoad() {
	fmt.Println(sink.FetchState())
}

// Attempt to populate the DB with all that ZD goodness
func TestDataImport(t *testing.T) {
	start := sink.FetchState()
	// Initialize Meta tables
	source.ListTicketFields(sink.ImportTicketFields)
	source.ListGroups(sink.ImportGroups)
	// Populate actual data
	sink.CommitSequence("organization_export", source.ExportOrganizations(start["organization_export"], sink.ImportOrganizations))
	sink.CommitSequence("user_export", source.ExportUsers(start["user_export"]+1, sink.ImportUsers))
	sink.CommitSequence("ticket_export", source.ExportTickets(start["ticket_export"], sink.ImportTickets))
	source.ListTicketMetrics(sink.ImportTicketMetrics)
	// Necessary evil until I have more time
	testRaw()
	testDataLoad()
}

func init() {
	cFile, err := os.Open("./exclude/conf.json")
	maybeFatal(err)

	maybeFatal(json.NewDecoder(cFile).Decode(&conf))

	sink = mysql.Open(conf.DBconf)
	source = zendesk.Open(http.DefaultClient, conf.ZDconf)
}

func maybeFatal(err error) {
	if err != nil {
		log.Fatal("Fatal:", err)
	}
}
