package zendb

import (
	"encoding/json"
	"github.com/rnpridgeon/zendb/provider/mysql"
	"github.com/rnpridgeon/zendb/provider/zendesk"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
	"github.com/rnpridgeon/zendb/models"
	"strings"
)

// TODO: make provider interface
var (
	conf Config
	sink *mysql.MysqlProvider
	source *zendesk.ZDProvider
	requireMetrics []int64
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
		SET tickets.priority = ticket_metadata.transformed_value
		WHERE ticket_fields.title = "Case Priority"`

	insertComponent =`
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.component = ticket_metadata.transformed_value
		WHERE ticket_fields.title = "Component"`

	insertVersion =`
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.version = ticket_metadata.raw_value
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

func transformComponent(obj interface{}) {
	entity := obj.(*models.Custom_fields)
	//TODO: Component, build cache pull from there
	var component int64 = 33020448
	if entity.Id == component && entity.Value != nil {
		val := entity.Value.(string)
		switch {
		case strings.Contains(val, "c3") || strings.Contains(val, "confluent_control_center"):
			entity.Transformed = "c3"
		case strings.Contains(val, "broker"):
			entity.Transformed="broker"
		case strings.Contains(val, "auto_data_balancer"):
			entity.Transformed = "adb"
		case strings.Contains(val, "_jms_"):
			entity.Transformed = "clients-jms"
		case strings.Contains(val, "python_"):
			entity.Transformed = "clients-python"
		case strings.Contains(val, "client_net"):
			entity.Transformed = "clients-dotNET"
		case strings.Contains(val, "_c_"):
			entity.Transformed = "clients-c/c++"
		case strings.Contains(val, "_go_"):
			entity.Transformed = "clients-golang"
		case strings.Contains(val, "third-party"):
			entity.Transformed = "clients-third-party"
		case strings.Contains(val, "java_"):
			entity.Transformed = "clients-java"
		case strings.Contains(val, "java_"):
			entity.Transformed = "clients-java"
		default:
			entity.Transformed = entity.Value.(string)
		}
	}
}

func transformPriority(obj interface{}) {
	entity := obj.(*models.Custom_fields)
	// TODO: Case Priority , build cache, pull from there
	var priority int64 = 33471847
	if entity.Id == priority && entity.Value != nil {
		if len(entity.Value.(string)) >=2 {
			entity.Transformed = entity.Value.(string)[:2]
		}
	}
}

func buildMetricsList(obj interface{}) {
	requireMetrics = append(requireMetrics, obj.(*models.Ticket).Id)
}

// Test custom query/post processing
func PostProcessing() {
	defer TimeTrack(time.Now(), "Ticket post processing")

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

func InitialLoad() {
	sink.RegisterTransformation("ticket_metadata", transformComponent)
	sink.RegisterTransformation("ticket_metadata", transformPriority)
	sink.RegisterTransformation("tickets", buildMetricsList)

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
	log.Printf("INFO: Fetching ticket metric updates since ticket id %d...\n", start["ticket_metrics"])
	//source.ExportTicketMetrics(requireMetrics , sink.ImportTicketMetrics)
	log.Printf("INFO: Fetching ticket audits since audit id %d", start["ticket_audit"])
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
