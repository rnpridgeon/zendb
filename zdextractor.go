package main

import (
	"log"
	"os"
	"strconv"

	"github.com/rnpridgeon/utils/configuration"
	"github.com/rnpridgeon/zendb/models"
	"github.com/rnpridgeon/zendb/provider/mysql"
	"github.com/rnpridgeon/zendb/provider/zendesk"
	"fmt"
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
		fields[obj.(models.OrganizationFields).SKey] = obj.(models.OrganizationFields).Id
		return obj
	}

}

func indexUserFields(fields map[string]int64) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		fields[obj.(models.UserFields).SKey] = obj.(models.UserFields).Id
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
func extractTicketFieldValues(lookup map[int64]string, fields *[]models.TicketData) func(interface{}) interface{} {
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
func extractOrganizationFieldValues(lookup map[string]int64, fields *[]models.OrganizationData) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		record := obj.(models.Organization)

		for k, v := range record.CustomFields {
			tmp := new(models.OrganizationData)
			tmp.ObjectID = record.Id
			tmp.Id = lookup[k]
			tmp.Title = k
			tmp.Value = v
			*fields = append(*fields, *tmp)
		}

		return obj
	}
}

//Capture custom ticket fields, put them in a table
func extractUserFieldValues(lookup map[string]int64, fields *[]models.UserData) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		record := obj.(models.User)

		for k, v := range record.CustomFields {
			tmp := new(models.UserData)
			tmp.ObjectID = record.Id
			tmp.Id = lookup[k]
			tmp.Title = k
			tmp.Value = v
			*fields = append(*fields, *tmp)
		}

		return obj
	}
}

// skip non-time tracking events
func extractChangeEvents(lookup map[int64]string, events *[]models.ChangeEvent) func(interface{}) interface{} {
	return func(obj interface{}) interface{} {
		e := obj.(models.Audit)
		keep := false
		//Time tracker id, annoying it can't be picked out of zd with a name
		for idx, _ := range e.Events {
			fieldID, _ := strconv.ParseInt(e.Events[idx].FieldName, 10, 0)
			if e.Events[idx].Type == "Change" && lookup[fieldID] == "Total time spent (sec)" {
				keep = true
				e.Events[idx].AuditId = e.Id
				*events = append(*events, e.Events[idx])
				continue
			}
		}
		if keep == true {
			return obj
		}
		return nil
	}
}

//func TestDispatch(t *testing.T) {
func main() {
	var (
		needsUpdate             []int64
		metricUpdates           []models.TicketMetric
		auditUpdates            []models.Audit
		ticketFieldValues       []models.TicketData
		organizationFieldValues []models.OrganizationData
		userFieldValues         []models.UserData
		auditEvents             []models.ChangeEvent
	)

	ticketFields := make(map[int64]string)
	OrganizationFields := make(map[string]int64)
	UserFields := make(map[string]int64)

	//configuration.FromFile("./exclude/conf.json")(&conf)

	if len(os.Args) != 2 {
		log.Fatalf("USAGE ./%s [path to config file]\n", os.Args[0])
	}

	configuration.FromFile(os.Args[1])(&conf)

	sink = mysql.Open(conf.DBconf)
	source = zendesk.Open(conf.ZDconf, requestQueue)

	stop := zendesk.NewDispatcher(requestQueue)

	sink.RegisterTransformation("OrganizationFields", indexOrganizationFields(OrganizationFields))
	sink.RegisterTransformation("Organization", extractOrganizationFieldValues(OrganizationFields, &organizationFieldValues))

	sink.RegisterTransformation("UserFields", indexUserFields(UserFields))
	sink.RegisterTransformation("User", extractUserFieldValues(UserFields, &userFieldValues))

	sink.RegisterTransformation("Ticket", buildMetricsList(&needsUpdate))
	sink.RegisterTransformation("Ticket", extractTicketFieldValues(ticketFields, &ticketFieldValues))
	sink.RegisterTransformation("TicketFields", indexTicketFields(ticketFields))

	sink.RegisterTransformation("Audit", extractChangeEvents(ticketFields, &auditEvents))

	source.ExportTicketFields(sink.ImportTicketFields)
	source.ExportOrganizationFields(sink.ImportOrganizationFields)
	source.ExportUserFields(sink.ImportUserFields)

	source.ExportGroups(sink.ImportGroups)
	source.ExportOrganizations(sink.ImportOrganizations, sink.FetchOffset("organization"))
	zendesk.WG.Wait()

	source.ExportUsers(sink.ImportUsers, sink.FetchOffset("user"))
	zendesk.WG.Wait()

	source.ExportTickets(sink.ImportTickets, sink.FetchOffset("ticket"))
	zendesk.WG.Wait()

	source.ExportCSAT(sink.ImportCSAT, sink.FetchOffset("satisfactionrating"))

	// Amortize the cost of prepared statements by batching individual requests
	for _, i := range needsUpdate {
		source.FetchAudits(i, auditAccumulator(&auditUpdates))
		source.FetchMetrics(i, metricAccumulator(&metricUpdates))
	}

	zendesk.WG.Wait()

	sink.ImportOrganizationCustomFields(organizationFieldValues)
	sink.ImportUserCustomFields(userFieldValues)
	sink.ImportTicketCustomFields(ticketFieldValues)

	sink.ImportAudit(auditUpdates)
	sink.ImportAuditChangeEvent(auditEvents)
	sink.ImportTicketMetrics(metricUpdates)

	// stop the dispatcher
	stop <- struct{}{}

	//TODO: The audits endpoint is spotty, post processing with a query can spot holes need to add method for filling them
	for err := source.Errors.Deque(); err != nil; err = source.Errors.Deque() {
		log.Printf("WARN: An error occurred fetching %+v\n", err)
	}

	for err := sink.Errors.Deque(); err != nil; err = source.Errors.Deque() {
		log.Printf("WARN: An error occurred persisting %+v\n", err)
	}
}
