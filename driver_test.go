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
	"time"
	"github.com/rnpridgeon/zendb/models"
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

	insertTTFR = `
		UPDATE tickets
			JOIN ticket_metrics on tickets.id = ticket_metrics.ticket_id
		SET tickets.ttfr = ticket_metrics.ttfr`

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
func PostProcessing() {
	defer TimeTrack(time.Now(), "Ticket Metrics import")

	sink.ExecRaw(insertPriority)
	sink.ExecRaw(insertComponent)
	sink.ExecRaw(insertVersion)
	sink.ExecRaw(insertSolved)
	sink.ExecRaw(insertTTFR)
}

func TestScheduled(t *testing.T) {
	InitialLoad()

	scheduler := NewScheduler(1 * MINUTE, Process)
	// Kill scheduler
	go func() {
		time.Sleep( 1 * MINUTE)
		scheduler.Stop()
	}()
	scheduler.Start()
}

func exportOrganizations() {
	e := sink.ExportOrganizations(0)
	for i := 0; i < len(e); i++ {
		fmt.Println(e[i])
	}
}

func exportTickets(orgId int64) {
	e := sink.ExportTickets(0, orgId)
	for i := 0; i < len(e); i++ {
		fmt.Printf("%+v\n", e[i])
	}
}

func processAudit(a []models.Audit) {
	fmt.Printf("%+v\n", a)
}

func InitialLoad() {
	source.ListTicketFields(sink.ImportTicketFields)
	source.ListGroups(sink.ImportGroups)
	Process()
}

func Process() {
	start := sink.FetchState()
	log.Printf("%+v\n", start)
	log.Printf("INFO: Fetching organization updates %v...\n", time.Unix(start["organization_export"],0))
	sink.CommitSequence("organization_export", source.ExportOrganizations(start["organization_export"], sink.ImportOrganizations))
	log.Printf("INFO: Fetching User updates since %v...\n",time.Unix(start["user_export"],0) )
	sink.CommitSequence("user_export", source.ExportUsers(start["user_export"], sink.ImportUsers))
	log.Printf("INFO: Fetching ticket updates since %v...\n", time.Unix(start["ticket_export"],0))
	sink.CommitSequence("ticket_export", source.ExportTickets(start["ticket_export"], sink.ImportTickets))
	log.Printf("INFO: Fetching ticket metric updates since %v...\n", time.Unix(start["ticket_metrics"],0))
	source.ExportTicketMetrics(start["ticket_metrics"], start["tickets"] , sink.ImportTicketMetrics)
	source.ExportTicketAudits(start["ticket_audit"], sink.ImportAudit)
	PostProcessing()
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
