package zendesk

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/valyala/fasthttp"
	"log"
)

const versionOverhead = 7

var (
	client = &fasthttp.Client{
		DisableHeaderNamesNormalizing: true,
	}
)

func Fetch(req *fasthttp.Request, resp *fasthttp.Response) (err error) {
	if len(req.Host()) == 0 {
		log.Fatal("Missing host %s", req.URI())
	}
	err = client.Do(req, resp)

	if err != nil {
		return &FetchError{err: err.Error(), shouldRetry: false, requestURI: req.RequestURI()}
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return &FetchError{err: fmt.Sprintf("ERROR Received %d status code for %s: ",
			resp.StatusCode(), req.URI().String()), //, response.Body()),
			statusCode: resp.StatusCode(),
		}
	}

	return err
}

func SetBasicAuth(request *fasthttp.Request, user string, password string) {
	auth := []byte(user + ":" + password)
	request.Header.Set("Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
}

type FetchError struct {
	shouldRetry bool
	statusCode  int
	requestURI  []byte
	err         string
}

func (f *FetchError) Error() string {
	return f.err
}

//func getVersion(req *fasthttp.Request) []byte {
//	return req.URI().Path()[:VERSION_OVERHEAD]
//}

func getOption(req *fasthttp.Request) []byte {
	path := req.URI().Path()
	return path[versionOverhead:bytes.LastIndexByte(path, '/')]
}

func getResource(req *fasthttp.Request) []byte {
	path := req.URI().Path()
	return path[bytes.LastIndexByte(path, '/'):]
}

func isExport(b []byte) bool {
	return bytes.Compare(b, []byte("/incremental")) == 0
}
