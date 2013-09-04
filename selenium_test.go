package selenium

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// client is the Selenium client being tested.
	client WebDriver

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

// setup sets up a test HTTP server along with a WebDriver that is
// configured to talk to that test server.  Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"sessionId": "123"}`)
	})

	// selenium client configured to use test server
	var err error
	client, err = NewRemote(caps, server.URL)
	if err != nil {
		panic("NewRemote: " + err.Error())
	}
}

// teardown closes the test HTTP server.
func teardown() {
	server.Close()
}

func testMethod(t *testing.T, r *http.Request, want string) {
	if want != r.Method {
		t.Errorf("Request method = %v, want %v", r.Method, want)
	}
}

func testHeader(t *testing.T, r *http.Request, header string, want string) {
	if value := r.Header.Get(header); want != value {
		t.Errorf("Header %s = %s, want: %s", header, value, want)
	}
}
