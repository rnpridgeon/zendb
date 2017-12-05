package zendesk

import (
	"encoding/json"
	"fmt"
	"github.com/rnpridgeon/zendb/models"
	"log"
	"net/http"
	"strconv"
	"time"
	"sync"
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
	sync.Pool
}

func Open(client *http.Client, conf *ZendeskConfig) (handle *ZDProvider) {
	httpClient = client
	return newHandler(conf)
}

func newHandler(conf *ZendeskConfig) (handle *ZDProvider) {
	handle = &ZDProvider{
		sync.Pool{New: func() interface{} {
			req, _ := http.NewRequest("GET", fmt.Sprintf(base, conf.Subdomain), nil)
			req.Header = header
			req.SetBasicAuth(conf.User, conf.Password)
			return req
		}},
	}

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
		if resp == nil {
			log.Println(err)
			return
		}
		log.Printf("ERROR: Unable to fetch from fetch from %s: %s", request.URL, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(obj); err != nil {
		log.Printf("Failed to decode %s: \n\t%s)", obj, err)
	}

	resp.Body.Close()
}

func (p *ZDProvider) ExportTicketFields(process func([]models.Ticket_field)) (last int64) {
	defer timeTrack(time.Now(), "Export Metrics")

	r := p.Get().(*http.Request)
	r.URL, _ = r.URL.Parse("./ticket_fields.json")

	defer  func() {
		r.URL, _ = r.URL.Parse("./")
		p.Put(r)
	}()

	var rezponze struct {
		pager
		Payload []models.Ticket_field `json:"ticket_fields"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r, &rezponze)

		process(rezponze.Payload)

		if rezponze.Next != "" {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	return rezponze.Payload[len(rezponze.Payload)-1].Id
}

func (p *ZDProvider) ExportTicketMetrics(toProcess []int64, process func([]models.Ticket_metrics)) (last int64) {
	defer timeTrack(time.Now(), "Export Metrics")

	r := p.Get().(*http.Request)

	defer func() {
		p.Put(r)
	}()

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
		deserialize(r, &rezponze)
		if rezponze.Payload.Ticket_id > 0 {
			payload[index] = rezponze.Payload
			index++
		}
		r.URL, _ = r.URL.Parse("../../")
	}

	process(payload[:index])

	return index
}

func (p *ZDProvider) ExportGroups(process func([]models.Group)) (last int64) {
	defer timeTrack(time.Now(), "Export Groups")

	// ensure url gets cleaned up and request is put back in the queue
	r := p.Get().(*http.Request)
	defer  func() {
		r.URL, _ = r.URL.Parse("./")
		p.Put(r)
	}()

	r.URL, _ = r.URL.Parse("./groups.json")

	var rezponze struct {
		pager
		Payload []models.Group `json:"groups"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r, &rezponze)

		process(rezponze.Payload)
		if rezponze.Next != "" {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	return 0
}

func (p *ZDProvider) ExportOrganizations(since int64, process func([]models.Organization)) (last int64) {
	defer timeTrack(time.Now(), "Export Organizations")
	r := p.Get().(*http.Request)

	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/organizations.json?start_time=%s",
		strconv.FormatInt(since, 10)))

	defer func(r *http.Request){
		r.URL, _ = r.URL.Parse("../")
		p.Put(r)
	}(r)

	var rezponze struct {
		pager
		Payload []models.Organization `json:"organizations"`
	}

	//iterate over pages, TODO: this needs to be moved out and cleaned up to keep things DRY
	for {
		deserialize(r, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	return rezponze.End
}

func (p *ZDProvider) GetOrganization(id int64, process func(organization models.Organization)) {
	defer timeTrack(time.Now(), "Export Organizations")

	r := p.Get().(*http.Request)
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./organizationss/%s.json", strconv.FormatInt(id, 10)))

	defer func(r *http.Request) {
		r.URL, _ = r.URL.Parse("../")
		p.Put(r)
	}(r)

	var payload models.Organization
	deserialize(r, &payload)

	process(payload)

}

func (p *ZDProvider) ExportUsers(since int64, process func([]models.User)) (last int64) {
	defer timeTrack(time.Now(), "Export Users")

	r := p.Get().(*http.Request)
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/users.json?start_time=%s",
		strconv.FormatInt(since, 10)))

	defer func(r *http.Request){
		r.URL, _ = r.URL.Parse("../")
		p.Put(r)
	}(r)

	var rezponze struct {
		pager
		Payload []models.User `json:"users"`
	}

	//iterate over pages, TODO: this needs to be moved out to keep things DRY
	for {
		deserialize(r, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	return rezponze.End
}

func (p *ZDProvider) ExportTickets(since int64, process func([]models.Ticket)) (last int64) {
	defer timeTrack(time.Now(), "Export Tickets")

	r := p.Get().(*http.Request)
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./incremental/tickets.json?start_time=%d", since))

	defer func(r *http.Request){
		r.URL, _ = r.URL.Parse("../")
		p.Put(r)
	}(r)

	var rezponze struct {
		pager
		Payload []models.Ticket `json:"tickets"`
	}

	//iterate over pages, TODO: this needs to be moved out to keep things DRY
	for {
		deserialize(r, &rezponze)

		process(rezponze.Payload)
		if rezponze.Count >= 1000 {
			r.URL, _ = r.URL.Parse(rezponze.Next)
			rezponze.Next = ""
			continue
		}
		break
	}

	return rezponze.End
}

func (p *ZDProvider) GetTicket(id int64, process func(ticket models.Ticket)) {
	defer timeTrack(time.Now(), "Export Tickets")

	r := p.Get().(*http.Request)
	r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d.json", id))

	defer func(r *http.Request){
		r.URL, _ = r.URL.Parse("../")
		p.Put(r)
	}(r)

	var payload models.Ticket

	deserialize(r, &payload)
	process(payload)
}

func (p *ZDProvider) ExportTicketAudits(toProcess []int64, process func(to []models.Audit)) (last int64) {
	defer timeTrack(time.Now(), "Export Audits")

	r := p.Get().(*http.Request)

	defer func(r *http.Request) {
		p.Put(r)
	}(r)

	if len(toProcess) == 0 {
		return 0
	}

	var rezponze struct {
		Payload []models.Audit `json:"audits"`
	}

	var index int64 = 0
	for _, item := range toProcess {
		r.URL, _ = r.URL.Parse(fmt.Sprintf("./tickets/%d/audits.json", item))
		deserialize(r, &rezponze)
		if len(rezponze.Payload) > 0 {
			process(rezponze.Payload)
		}
		r.URL, _ = r.URL.Parse("../../")
	}

	return index
}

func init() {
	header = make(http.Header)

	header.Add("Accept", "application/json")
	header.Add("Content-Type", "application/json")
	header.Add("Accept-Charset", "utf-8")
	header.Add("Accept-Language", "en-US")

}
