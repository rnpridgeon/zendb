package zendb

import (
	"encoding/json"
	"github.com/rnpridgeon/zendb/models"
	"github.com/rnpridgeon/zendb/provider/mysql"
	"github.com/rnpridgeon/zendb/provider/zendesk"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"sync"
)

// TODO: make provider interface
var (
	conf   Config
	sink   *mysql.MysqlProvider
	source *zendesk.ZDProvider

	needsUpdate  []int64
	ticketFields = make(map[string]int64)
)

type Config struct {
	ZDconf *zendesk.ZendeskConfig `json:"zendesk"`
	DBconf *mysql.MysqlConfig     `json:"database"`
}

const (
	insertPriority = `
	UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.priority = ticket_metadata.transformed_value
		WHERE ticket_fields.title = "Case Priority"`

	insertComponent = `
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.component = ticket_metadata.transformed_value
		WHERE ticket_fields.title = "Component"`

	insertVersion = `
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.version = ticket_metadata.raw_value
		WHERE ticket_fields.title like "%Kafka Version"`

	insertRCA = `
		UPDATE tickets
			JOIN ticket_metadata on tickets.id = ticket_metadata.ticket_id
			JOIN ticket_fields on field_id = ticket_fields.id
		SET tickets.cause = ticket_metadata.raw_value
		WHERE ticket_fields.title like "Root Cause"`

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
			entity.Transformed = "broker"
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
		if len(entity.Value.(string)) >= 2 {
			entity.Transformed = entity.Value.(string)[:2]
		}
	}
}

// Capture/Index ticket fields for ease of use in processing
func captureFieldList(obj interface{}) {
	ticketFields[obj.(*models.Ticket_field).Title] = obj.(*models.Ticket_field).Id
}

//Capture ticket id for each ticket processed by the sink
func buildMetricsList(obj interface{}) {
	needsUpdate = append(needsUpdate, obj.(*models.Ticket).Id)
}

// Test custom query/post processing
func PostProcessing() {
	defer TimeTrack(time.Now(), "Ticket post processing")
	//reset array
	needsUpdate = nil

	sink.ExecRaw(insertPriority)
	sink.ExecRaw(insertComponent)
	sink.ExecRaw(insertVersion)
	sink.ExecRaw(insertRCA)
	//sink.ExecRaw(insertSolved)
	//sink.ExecRaw(insertTTFR)
}

func TestScheduled(t *testing.T) {
	InitialLoad()
	s := NewScheduler(8 * SECOND, Process)

	go func() {
		time.Sleep(10 * SECOND)
		s.Stop()
	}()

	s.Start()
	return
}

func InitialLoad() {
	sink.RegisterTransformation("ticket_fields", captureFieldList)
	sink.RegisterTransformation("ticket_metadata", transformComponent)
	sink.RegisterTransformation("ticket_metadata", transformPriority)
	sink.RegisterTransformation("tickets", buildMetricsList)

	source.ExportTicketFields(sink.ImportTicketFields)
	source.ExportGroups(sink.ImportGroups)
}

func Process() {
	start := sink.FetchState()

	log.Printf("INFO: Fetching organization updates %v...\n", time.Unix(start["organization_export"], 0))
	sink.CommitSequence("organization_export", source.ExportOrganizations(start["organization_export"], sink.ImportOrganizations))

	log.Printf("INFO: Fetching User updates since %v...\n", time.Unix(start["user_export"], 0))
	sink.CommitSequence("user_export", source.ExportUsers(start["user_export"], sink.ImportUsers))

	log.Printf("INFO: Fetching ticket updates since %v...\n", time.Unix(start["ticket_export"], 0))
	sink.CommitSequence("ticket_export", source.ExportTickets(start["ticket_export"], sink.ImportTickets))

	var wg sync.WaitGroup
	wg.Add(2)
	log.Printf("INFO: Fetching ticket metric updates since ticket id %d...\n", start["ticket_export"])
	go func(){
		source.ExportTicketMetrics(needsUpdate, sink.ImportTicketMetrics)
		wg.Done()
	}()

	log.Printf("INFO: Fetching ticket audits since audit id %d", start["ticket_audit"])
	go func() {
		source.ExportTicketAudits(needsUpdate, sink.ImportAudit)
		wg.Done()
	}()
	wg.Wait()
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
