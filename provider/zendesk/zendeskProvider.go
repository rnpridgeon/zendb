package zendesk

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/rnpridgeon/utils/collections"
)

const (
	base      = "https://%s.zendesk.com/"
	ZDVersion = "api/v2/"
)

type ZendeskConfig struct {
	User      string
	Password  string
	Subdomain string
}

type ZDProvider struct {
	user         string
	password     string
	host         string
	requestQueue chan *Task
	Errors       *collections.DEQueue
}

func Open(conf *ZendeskConfig, requestQueue chan *Task) (handle *ZDProvider) {
	return newZendeskSource(conf, requestQueue)
}

func newZendeskSource(conf *ZendeskConfig, requestQueue chan *Task) (handle *ZDProvider) {
	handle = &ZDProvider{
		user:         conf.User,
		password:     conf.Password,
		host:         fmt.Sprintf(base, conf.Subdomain),
		requestQueue: requestQueue,
		Errors:       collections.NewDEQueue(),
	}
	return handle
}

func (p *ZDProvider) runTask(endpoint string, query string, onSuccess func(interface{})) {
	req := p.getRequest()
	req.URI().SetPath(endpoint)
	req.URI().SetQueryString(query)

	task := AcquireTask(p.Errors)

	onFailure := func(err error) {
		log.Printf("And error occurred %v", err)
	}
	task.req = req
	task.onSuccess = onSuccess
	task.onFailure = onFailure

	p.requestQueue <- task

	//HACK: forces go routine off runnable queue to ensure waiters are incremented before returning
	time.Sleep(1 * time.Millisecond)
}

func (p *ZDProvider) getRequest() (req *fasthttp.Request) {
	req = fasthttp.AcquireRequest()
	SetBasicAuth(req, p.user, p.password)
	req.SetRequestURI(p.host)
	return req
}

func (p *ZDProvider) ExportGroups(onSuccess func(interface{})) {
	p.runTask(ZDVersion+"groups.json", "", onSuccess)
}

func (p *ZDProvider) ExportOrganizations(onSuccess func(interface{}), since int64) {
	p.runTask(ZDVersion+"incremental/organizations.json", "start_time="+strconv.FormatInt(since, 10), onSuccess)
}

func (p *ZDProvider) ExportOrganizationFields(onSucess func(interface{})) {
	p.runTask(ZDVersion+"organization_fields.json", "", onSucess)
}

func (p *ZDProvider) ExportUsers(onSuccess func(interface{}), since int64) {
	p.runTask(ZDVersion+"incremental/users.json", "start_time="+strconv.FormatInt(since, 10), onSuccess)
}

func (p *ZDProvider) ExportUserFields(onSucess func(interface{})) {
	p.runTask(ZDVersion+"user_fields.json", "", onSucess)
}

func (p *ZDProvider) ExportTickets(onSuccess func(interface{}), since int64) {
	p.runTask(ZDVersion+"incremental/tickets.json", "start_time="+strconv.FormatInt(since, 10), onSuccess)
}

func (p *ZDProvider) ExportTicketFields(onSucess func(interface{})) {
	p.runTask(ZDVersion+"ticket_fields.json", "", onSucess)
}

func (p *ZDProvider) FetchAudits(ticketID int64, onSuccess func(interface{})) {
	p.runTask(ZDVersion+"tickets/"+strconv.FormatInt(ticketID, 10)+"/audits.json", "", onSuccess)
}

func (p *ZDProvider) FetchMetrics(ticketID int64, onSuccess func(interface{})) {
	p.runTask(ZDVersion+"tickets/"+strconv.FormatInt(ticketID, 10)+"/metrics.json", "", onSuccess)
}
