package zendb

import (
	//"os"
	"fmt"
	"log"
	"strconv"
	"testing"

	"github.com/rnpridgeon/utils/configuration"
	"github.com/rnpridgeon/zendb/models"
	"github.com/rnpridgeon/zendb/provider/mysql"
	"github.com/rnpridgeon/zendb/provider/zendesk"
)

// TODO: make provider interface
var (
	conf   Config
	sink   *mysql.MysqlProvider
	source *zendesk.ZDProvider

	requestQueue = make(chan *zendesk.Task, 100)
)

type Config struct {
	ZDconf *zendesk.ZendeskConfig `json:"zendesk"`
	DBconf *mysql.MysqlConfig     `json:"database"`
}

func auditAccumulator(accumulator *[]models.Audit) func(interface{}) {
	return func(dat interface{}) {
		*accumulator = append(*dat.(*[]models.Audit), *accumulator...)
	}
}

func metricAccumulator(accumulator *[]models.TicketMetric) func(interface{}) {
	return func(dat interface{}) {
		*accumulator = append(*accumulator, *dat.(*models.TicketMetric))
	}
}

// Index ticket custom ticket fields
func indexTicketFields(fields map[int64]string) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		fields[obj.(models.TicketFields).Id] = obj.(models.TicketFields).Title
		return obj
	}

}

func indexOrganizationFields(fields map[string]int64) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		fields[obj.(models.OrganizationFields).Title] = obj.(models.OrganizationFields).Id
		return obj
	}

}

func indexUserFields(fields map[string]int64) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		fields[obj.(models.UserFields).Title] = obj.(models.UserFields).Id
		return obj
	}

}

//Capture ticket id for each ticket processed by the sink
func buildMetricsList(needsUpdate *[]int64) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		*needsUpdate = append(*needsUpdate, obj.(models.Ticket).Id)
		return obj
	}
}

//Capture custom ticket fields, put them in a table
func persistTicketFieldValues(lookup map[int64]string, fields *[]models.TicketData) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		record := obj.(models.Ticket)
		for idx := range record.CustomFields {
			record.CustomFields[idx].ObjectID = record.Id
			record.CustomFields[idx].Title = lookup[record.CustomFields[idx].Id]
		}
		*fields = append(*fields, record.CustomFields...)

		return obj
	}
}

//Capture custom ticket fields, put them in a table
func persistOrganizationFieldValues(lookup map[string]int64, fields *[]models.OrganizationData) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		record := obj.(models.Organization)

		for k, v := range record.CustomFields {
			tmp := models.OrganizationData{}
			tmp.ObjectID  = record.Id
			tmp.Id = lookup[k]
			tmp.Title = k
			tmp.Value = v
			*fields = append(*fields, tmp)
		}

		return obj
	}
}

//Capture custom ticket fields, put them in a table
func persistUserFieldValues(lookup map[string]int64, fields *[]models.UserData) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		record := obj.(models.User)

		for k, v := range record.CustomFields {
			tmp := models.UserData{}
			tmp.ObjectID  = record.Id
			tmp.Id = lookup[k]
			tmp.Title = k
			tmp.Value = v
			*fields = append(*fields, tmp)
		}

		return obj
	}
}

// skip non-time tracking events
func filterAudits(lookup map[int64]string) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered from panic processing ", obj)
			}
		}()
		type Audit struct {
			Id        int64 `structs:",isKey"`
			Ticketid  int64 `structs:",isKey"`
			Createdat models.Utime
			Authorid  int64
			Value     interface{}
		}
		e := obj.(models.Audit)
		//Time tracker id, annoying it can't be picked out of zd with a name
		for _, se := range e.Events {
			fieldID, _ := strconv.ParseInt(se.FieldName, 10, 0)
			if se.Type == "Change" && lookup[fieldID] == "Total time spent (sec)" {
				return &Audit{e.Id, e.TicketId, e.CreatedAt, e.AuthorId, se.Value}
			}
		}
		return nil
	}
}

func TestDispatch(t *testing.T) {
	var needsUpdate []int64
	var MetricUpdates []models.TicketMetric
	var AuditUpdates []models.Audit
	var ticketFieldValues []models.TicketData
	var OrganizationFieldValues []models.OrganizationData
	var UserFieldValues []models.UserData

	ticketFields := make(map[int64]string)
	OrganizationFields := make(map[string]int64)
	UserFields := make(map[string]int64)

	configuration.FromFile("./exclude/conf.json")(&conf)
	//TODO: Add actual runner
	//configuration.FromFile(os.Args[1])(&conf)

	sink = mysql.Open(conf.DBconf)
	source = zendesk.Open(conf.ZDconf, requestQueue)

	stop := zendesk.NewDispatcher(requestQueue)

	sink.RegisterTransformation("TicketFields", indexTicketFields(ticketFields))
	sink.RegisterTransformation("OrganizationFields", indexOrganizationFields(OrganizationFields))
	sink.RegisterTransformation("Organization", persistOrganizationFieldValues(OrganizationFields, &OrganizationFieldValues))

	sink.RegisterTransformation("UserFields", indexUserFields(UserFields))
	sink.RegisterTransformation("User", persistUserFieldValues(UserFields, &UserFieldValues))

	sink.RegisterTransformation("Ticket", buildMetricsList(&needsUpdate))
	sink.RegisterTransformation("Ticket", persistTicketFieldValues(ticketFields, &ticketFieldValues))

	sink.RegisterTransformation("Audit", filterAudits(ticketFields))

	source.ExportTicketFields(sink.ImportTicketFields)
	source.ExportOrganizationFields(sink.ImportOrganizationFields)
	source.ExportUserFields(sink.ImportUserFields)

	source.ExportGroups(sink.ImportGroups)
	source.ExportOrganizations(sink.ImportOrganizations, sink.FetchOffset("organization"))
	zendesk.WG.Wait()

	//source.ExportUsers(sink.ImportUsers, sink.FetchOffset("user"))
	zendesk.WG.Wait()

	source.ExportTickets(sink.ImportTickets, sink.FetchOffset("ticket"))
	zendesk.WG.Wait()

	// Amortize the cost of prepared statements by batching individual requests
	for _, i := range needsUpdate {
		source.FetchAudits(i, auditAccumulator(&AuditUpdates))
		source.FetchMetrics(i, metricAccumulator(&MetricUpdates))
	}

	// wait for audits and metrics to finish
	zendesk.WG.Wait()

	sink.ImportAudit(AuditUpdates)
	sink.ImportTicketMetrics(MetricUpdates)
	sink.ImportTicketCustomFields(ticketFieldValues)

	// stop the dispatcher
	stop <- struct{}{}

	for err := source.Errors.Deque(); err != nil; err = source.Errors.Deque() {
		log.Printf("WARN: An error occurred fetching %+v\n", err)
	}

	for err := sink.Errors.Deque(); err != nil; err = source.Errors.Deque() {
		log.Printf("WARN: An error occurred persisting %+v\n", err)
	}
}
