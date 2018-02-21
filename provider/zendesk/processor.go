package zendesk

import (
	"bytes"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/rnpridgeon/utils/encoding"
	"github.com/rnpridgeon/zendb/models"
)

func resourceFactory(resource string) (ret interface{}) {
	switch resource {
	case "groups":
		return &[]models.Groups{}
	case "organizations":
		return &[]models.Organization{}
	case "organization":
		return &models.Organization{}
	case "organization_fields":
		return &[]models.OrganizationFields{}
	case "users":
		return &[]models.User{}
	case "user":
		return &models.User{}
	case "user_fields":
		return &[]models.UserFields{}
	case "tickets":
		return &[]models.Ticket{}
	case "ticket":
		return &models.Ticket{}
	case "ticket_fields":
		return &[]models.TicketFields{}
	case "ticket_metric":
		return &models.TicketMetric{}
	case "audits":
		return &[]models.Audit{}
	default:
		return nil
	}
}

type RequestDescriptor struct {
	Payload  []byte
	Next     []byte
	Resource string
	Count    int64
}

func PreProcess(data []byte) *RequestDescriptor {
	reader := encoding.NewJSONStripper(data)
	reader.GetMembers()

	var rd = &RequestDescriptor{}
	for current := string(reader.NextMember()); len(current) > 0; {
		switch current {
		case "groups":
			fallthrough
		case "organization", "organizations", "organization_fields":
			fallthrough
		case "user", "users", "user_fields":
			fallthrough
		case "ticket", "tickets", "ticket_fields", "audits", "ticket_metric":
			rd.Resource = current
			rd.Payload = reader.NextMember()
		case "count":
			count, _ := strconv.ParseInt(string(reader.NextMember()), 10, 0)
			rd.Count = count
		case "previous_page":
		case "next_page":
			res := reader.NextMember()
			if bytes.Compare(res, []byte("previous_page")) != 0 {
				rd.Next = res
			}
		}
		current = string(reader.NextMember())
	}
	return rd
}

func (t *Task) Process(requestQueue chan *Task) {
	resp := fasthttp.AcquireResponse()

	//TODO: Implement delayed operation queue/dispatcher and actual error handling
	if err := Fetch(t.req, resp); err != nil {
		switch err.(*FetchError).statusCode {
		case 429:
			log.Printf("WARN: Request limit hit, adding %s to overflow queue\n", t.req.URI())
			time.Sleep(1 * time.Minute)
			requestQueue <- t
		case 404:
			log.Printf("WARN: No resource available for %s, aborting fetch\n", t.req.URI())
		default:
			log.Printf("ERROR: %v processing request %+v\n", err, t.req.URI())
			t.errors.Enqueue(t.req)
		}

		WG.Done()
		return
	}

	rd := PreProcess(resp.Body())

	//TODO: finish API parser build actual request builder
	if rd.Next != nil && !isExport(getOption(t.req)) || isExport(getOption(t.req)) && rd.Count >= 1000 {
		log.Printf("INFO: Placing uri %s on the queue\n", rd.Next)
		t.req.SetRequestURIBytes(rd.Next)
		requestQueue <- t
	} else {
		defer ReleaseTask(t)
	}

	resource := resourceFactory(rd.Resource)

	if err := json.Unmarshal(rd.Payload, resource); err == nil {
		t.onSuccess(resource)
	} else {
		t.errors.Enqueue(t)
		log.Printf("ERROR: Failed to deserialized payload %v \n", string(rd.Payload))
	}

	fasthttp.ReleaseResponse(resp)
	WG.Done()
}
