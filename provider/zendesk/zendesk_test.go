package zendesk

import (
	"bytes"
	"fmt"
	"github.com/rnpridgeon/utils/configuration"
	"github.com/valyala/fasthttp"
	"reflect"
	"sync"
	"testing"
)

// Defined in driver_test.go
var conf *ZendeskConfig

//
var testUris = [][]byte{
	[]byte("https://confluent.zendesk.com/api/v2/tickets.json"),
	[]byte("https://confluent.zendesk.com/api/v2/tickets/1234.json"),
	[]byte("https://confluent.zendesk.com/api/v2/incremental/tickets.json?start_time=0"),
	[]byte("https://confluent.zendesk.com/api/v2/ticket_fields.json"),
	[]byte("https://confluent.zendesk.com/api/v2/tickets/1234/audits.json"),
	[]byte("https://confluent.zendesk.com/api/v2/tickets/1234/metrics.json"),
	[]byte("https://confluent.zendesk.com/api/v2/groups.json"),
}

func TestURIParser(t *testing.T) {
	tests := []struct {
		uri      []byte
		extract  func(*fasthttp.Request) []byte
		expected []byte
	}{
		{uri: testUris[0], extract: getOption, expected: []byte("")},
		{uri: testUris[1], extract: getOption, expected: []byte("/tickets")},
		{uri: testUris[2], extract: getOption, expected: []byte("/incremental")},
		{uri: testUris[3], extract: getOption, expected: []byte("")},
		{uri: testUris[4], extract: getOption, expected: []byte("/tickets/1234")},
		{uri: testUris[5], extract: getResource, expected: []byte("/metrics.json")},
		{uri: testUris[6], extract: getResource, expected: []byte("/groups.json")},
	}

	for _, test := range tests {
		req := fasthttp.AcquireRequest()
		req.SetRequestURIBytes(test.uri)
		fmt.Printf("%s\n", getResource(req)[1:len(getResource(req))-5])
		if bytes.Compare(test.extract(req), test.expected) != 0 {
			t.Errorf("Failed to parse request URI: %s \n Got: %s, Expected %s\n",
				req.URI(), test.extract(req), test.expected)
		}
	}
}

func TestRequestPreprocessing(t *testing.T) {
	wg := sync.WaitGroup{}
	tests := []struct {
		uri          []byte
		resource     string
		nonZeroCount bool
		nonNilNext   bool
	}{
		{uri: testUris[0], resource: "tickets", nonZeroCount: true, nonNilNext: true},
		{uri: testUris[1], resource: "ticket", nonZeroCount: false, nonNilNext: false},
		{uri: testUris[2], resource: "tickets", nonZeroCount: true, nonNilNext: true},
		{uri: testUris[3], resource: "ticket_fields", nonZeroCount: true, nonNilNext: false},
		{uri: testUris[4], resource: "audits", nonZeroCount: true, nonNilNext: false},
		{uri: testUris[5], resource: "ticket_metric", nonZeroCount: false, nonNilNext: false},
		{uri: testUris[6], resource: "groups", nonZeroCount: true, nonNilNext: false},
	}

	for _, test := range tests {
		wg.Add(1)
		go func(test struct {
			uri          []byte
			resource     string
			nonZeroCount bool
			nonNilNext   bool
		}) {
			req := fasthttp.AcquireRequest()
			SetBasicAuth(req, conf.User, conf.Password)
			req.SetRequestURIBytes(test.uri)

			rd := testPreProcessing(req)
			fasthttp.ReleaseRequest(req)

			if !(rd.Resource == test.resource &&
				len(rd.Payload) != 0 &&
				rd.Next != nil == test.nonNilNext &&
				rd.Count != 0 == test.nonZeroCount) {
				t.Errorf("FAIL: TestStuff with resource: %s, count: %d,  next: %s, sizeof Payload: %d\n",
					rd.Resource, rd.Count, rd.Next, len(rd.Payload))
			}
			wg.Done()
		}(test)
	}
	wg.Wait()
}

func testPreProcessing(req *fasthttp.Request) *RequestDescriptor {
	resp := fasthttp.AcquireResponse()
	client := fasthttp.Client{}
	client.Do(req, resp)
	return PreProcess(resp.Body())
}

func TestFactory(t *testing.T) {
	wg := sync.WaitGroup{}
	tests := []struct {
		uri   []byte
		rType string
	}{
		{uri: testUris[0], rType: "[]models.Ticket"},
		{uri: testUris[1], rType: "models.Ticket"},
		{uri: testUris[2], rType: "[]models.Ticket"},
		{uri: testUris[3], rType: "[]models.Audit"},
		{uri: testUris[4], rType: "[]models.TicketMetrics"},
		{uri: testUris[5], rType: "[]models.Group"},
	}

	for _, test := range tests {
		wg.Add(1)
		go func(test struct {
			uri   []byte
			rType string
		}) {
			//Open(conf, requestQueue)
			req := fasthttp.AcquireRequest()
			SetBasicAuth(req, conf.User, conf.Password)
			req.SetRequestURIBytes(test.uri)

			rd := testPreProcessing(req)
			fasthttp.ReleaseRequest(req)

			obj := resourceFactory(rd.Resource)

			if reflect.TypeOf(obj).String() != test.rType {
				t.Errorf("FAILED: TestFactory expected: %s got: %s\n", reflect.TypeOf(obj).String(), test.rType)
			}
			wg.Done()
		}(test)
	}
	wg.Wait()
}

func init() {
	configuration.FromFile("../../exclude/test.json")(&conf)
}
