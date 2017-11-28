package zendesk

import (
	"github.com/rnpridgeon/zendb/models"
	"net/http"
	"strconv"
	"fmt"
	"log"
	"encoding/json"
	"time"
)

var (
	httpClient *http.Client
	header     http.Header
)

type pager struct {
	Next     string `json:"next_page"`
	Previous string `json:"previous_page"`
	End      int64  `json:"end_time"`
	Count    int64  `json:"count"`
}

const base = "https://%s.zendesk.com/api/v2/"

type ZendeskConfig struct {
	User      string
	Password  string
	Subdomain string
}

type ZDProvider struct {
	*http.Request
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("INFO: %s took %s", name, elapsed)
}

func deserialize(request *http.Request, object interface{}) {
	resp, err := httpClient.Do(request)

	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Unable to fetch from fetch from %s: %s", request.URL, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(object); err != nil {
		log.Printf("Failed to fetch from %s: \n\t%s)", request.URL, err)
	}

	resp.Body.Close()
}

func (r *ZDProvider) ListTicketFields(process func([]models.Ticket_field)) (last int64) {
	r.URL, _ = r.URL.Parse("./ticket_fields.json")

	var rezponze struct {
		pager
		Payload []models.Ticket_field `json:"ticket_fields"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)

		if rezponze.Next != "" {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	// clean-up
	r.URL, _ = r.URL.Parse("./")
	return rezponze.Payload[len(rezponze.Payload)-1].Id
}

func (r *ZDProvider) ExportTicketMetrics(since int64, end int64, process func([]models.Ticket_metrics)) (last int64) {
	payload := make([]models.Ticket_metrics, end-since + 1)

	var rezponze struct {
		Payload models.Ticket_metrics `json:"ticket_metric"`
	}

	var index int64
	for index = 0; since < end; since++{
		r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d/metrics.json", since))

		deserialize(r.Request, &rezponze)
		if rezponze.Payload.Ticket_id > 0 {
			payload[index] = rezponze.Payload
			index++
		}
		r.URL, _ = r.URL.Parse("../../")
	}

	process(payload[:index])
	return index
}



func (r *ZDProvider) ListGroups(process func([]models.Group)) (last int64) {
	r.URL, _ = r.URL.Parse("./groups.json")

	var rezponze struct {
		pager
		Payload []models.Group `json:"groups"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Next != "" {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}
	// clean-up
	r.URL, _ = r.URL.Parse("./")
	return rezponze.Payload[len(rezponze.Payload)-1].Id
}

func (r *ZDProvider) ExportOrganizations(since int64, process func([]models.Organization)) (last int64) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/organizations.json?start_time=%s",
		strconv.FormatInt(since, 10)))

	var rezponze struct {
		pager
		Payload []models.Organization `json:"organizations"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}
	// clean-up
	r.URL, _ = r.URL.Parse("../")
	return rezponze.End
}

func (r *ZDProvider) GetOrganization(id int64, process func(organization models.Organization)) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./organizationss/%s.json",strconv.FormatInt(id, 10)))

	var payload models.Organization
	deserialize(r.Request, &payload)
	process(payload)

	r.URL, _ = r.URL.Parse("../")
}

func (r *ZDProvider) ExportUsers(since int64, process func([]models.User)) (last int64) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/users.json?start_time=%s",
		strconv.FormatInt(since, 10)))

	var rezponze struct {
		pager
		Payload []models.User `json:"users"`
	}

	//iterate over pages, TODO: this needs to be moved out to keep things DRY
	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	// clean-up
	r.URL, _ = r.URL.Parse("../")
	return rezponze.End
}

func (r *ZDProvider) ExportTickets(since int64, process func([]models.Ticket)) (last int64) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/tickets.json?start_time=%d", since))

	var rezponze struct {
		pager
		Payload []models.Ticket `json:"tickets"`
	}

	//iterate over pages, TODO: this needs to be moved out to keep things DRY
	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	// clean-up
	r.URL, _ = r.URL.Parse("../")
	return rezponze.End
}

func (r *ZDProvider) ExportTicketAudits(since int64, process func([]models.Audit)) (last string) {
	r.URL, _ = r.URL.Parse("./ticket_audits.json?cursor=")

	var rezponze struct {
		Before_URL			string			`json:"before_url"`
		Before_Cursor		string			`json:"before_cursor"`
		After_URL			string		 	`json:"after_url"`
		After_Cursor		string			`json:"after_cursor"`
		Payload 			[]models.Audit	`json:"audits"`
	}

	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Payload[0].Id < since || rezponze.Before_Cursor == "" {
			break;
		}
		r.URL, _ = r.URL.Parse(rezponze.Before_URL)
		rezponze.Before_Cursor = ""
	}

	// clean-up
	r.URL, _ = r.URL.Parse("./")
	return rezponze.After_Cursor
}

func (r *ZDProvider) GetTicket(id int64, process func(ticket models.Ticket)) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d.json",id))

	var payload models.Ticket
	deserialize(r.Request, &payload)
	process(payload)

	r.URL, _ = r.URL.Parse("../")
}

func newHandler(conf *ZendeskConfig) (handle *ZDProvider) {
	req, _ := http.NewRequest("GET", fmt.Sprintf(base, conf.Subdomain), nil)
	handle = &ZDProvider{req}
	handle.Header = header
	handle.SetBasicAuth(conf.User, conf.Password)

	return handle
}

func Open(client *http.Client, conf *ZendeskConfig) (handle *ZDProvider) {
	httpClient = client
	return newHandler(conf)
}

func init() {
	header = make(http.Header)

	header.Add("Accept", "application/json")
	header.Add("Content-Type", "application/json")
	header.Add("Accept-Charset", "utf-8")
	header.Add("Accept-Language", "en-US")

}
