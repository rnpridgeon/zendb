package zendesk

import (
	"encoding/json"
	"fmt"
	"github.com/rnpridgeon/zendb/models"
	"log"
	"net/http"
	"strconv"
	"time"
)

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("INFO: %s took %s", name, elapsed)
}

var (
	httpClient *http.Client
	header     http.Header
)

const base = "https://%s.zendesk.com/api/v2/"

type ZendeskConfig struct {
	User      string
	Password  string
	Subdomain string
}

type ZDProvider struct {
	*http.Request
}

func Open(client *http.Client, conf *ZendeskConfig) (handle *ZDProvider) {
	httpClient = client
	return newHandler(conf)
}

func newHandler(conf *ZendeskConfig) (handle *ZDProvider) {
	req, _ := http.NewRequest("GET", fmt.Sprintf(base, conf.Subdomain), nil)
	handle = &ZDProvider{req}
	handle.Header = header
	handle.SetBasicAuth(conf.User, conf.Password)

	return handle
}

type pager struct {
	Next     string `json:"next_page"`
	Previous string `json:"previous_page"`
	End      int64  `json:"end_time"`
	Count    int64  `json:"count"`
}

func deserialize(request *http.Request, obj interface{}) {
	resp, err := httpClient.Do(request)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Unable to fetch from fetch from %s: %s", request.URL, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(obj); err != nil {
		log.Printf("Failed to decode %s: \n\t%s)", obj, err)
	}

	resp.Body.Close()
}

func (r *ZDProvider) ExportTicketFields(process func([]models.Ticket_field)) (last int64) {
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

func (r *ZDProvider) ExportTicketMetrics(toProcess []int64, process func([]models.Ticket_metrics)) (last int64) {
	if len(toProcess) == 0 {
		return 0
	}

	payload := make([]models.Ticket_metrics, len(toProcess))

	var rezponze struct {
		Payload models.Ticket_metrics `json:"ticket_metric"`
	}

	var index int64 = 0
	for _, item := range toProcess {
		r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d/metrics.json", item))
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

func (r *ZDProvider) ExportGroups(process func([]models.Group)) (last int64) {
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
	//	return rezponze.Payload[len(rezponze.Payload)-1].Id
	return 0
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
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./organizationss/%s.json", strconv.FormatInt(id, 10)))

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

func (r *ZDProvider) GetTicket(id int64, process func(ticket models.Ticket)) {
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d.json", id))

	var payload models.Ticket
	deserialize(r.Request, &payload)
	process(payload)

	r.URL, _ = r.URL.Parse("../")
}

func (r *ZDProvider) ExportTicketAudits(toProcess []int64, process func(to []models.Audit)) (last string) {
	r.URL, _ = r.URL.Parse("./ticket_audits.json?cursor=")

	var rezponze struct {
		Next   		 string         `json:"before_url"`
		Previous     string         `json:"after_url"`
		Payload      []models.Audit `json:"audits"`
	}

	for {
		deserialize(r.Request, &rezponze)

		process(rezponze.Payload)
		if rezponze.Next == "" {
			break
		}
		r.URL, _ = r.URL.Parse(rezponze.Next)
		rezponze.Next = ""
	}

	// clean-up
	r.URL, _ = r.URL.Parse("./")
	return rezponze.Previous
}

func init() {
	header = make(http.Header)

	header.Add("Accept", "application/json")
	header.Add("Content-Type", "application/json")
	header.Add("Accept-Charset", "utf-8")
	header.Add("Accept-Language", "en-US")

}
