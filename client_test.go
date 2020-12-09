package runalyze

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewClient confirms that a client can be created with the default baseURL
// and default User-Agent.
func TestNewClient(t *testing.T) {
	c := NewClient(nil, "123456")

	assert.Equal(t, BaseURLV1, c.baseURL.String(), "should configure the client to use the default url")
	assert.Equal(t, userAgent, c.userAgent, "should configure the client to use the default user-agent")
	assert.Equal(t, "123456", c.apiToken, "should configure the client to use the API token")
}

// TestNewRequest confirms that NewRequest returns an API request with the
// correct URL, a correctly encoded body and the correct User-Agent and
// Content-Type headers set.
func TestNewRequest(t *testing.T) {
	c := NewClient(nil, "123456")

	t.Run("valid request", func(tc *testing.T) {

		inURL, outURL := "foo", BaseURLV1+"foo"
		date, _ := time.Parse(time.RFC3339, "2020-10-01T23:00:00Z")
		inBody, outBody := Sleep{DateTime: date, Duration: 102, RemDuration: 25}, `{"date_time":"2020-10-01T23:00:00Z","duration":102,"rem_duration":25}`+"\n"

		req, err := c.NewRequest("GET", inURL, inBody)

		assert.Nil(tc, err, "should not return an error")
		assert.Equal(tc, outURL, req.URL.String(), "should expand relative URLs to absolute URLs")

		body, _ := ioutil.ReadAll(req.Body)
		assert.Equal(tc, outBody, string(body), "should encode the request body as JSON")
		assert.Equal(tc, c.userAgent, req.Header.Get("User-Agent"), "should pass the correct user agent")
		assert.Equal(tc, "application/json", req.Header.Get("Content-Type"), "should set the content-type as application/json")
		assert.Equal(tc, "123456", req.Header.Get("token"), "should pass the API token in the headers")
	})

	t.Run("request with invalid JSON", func(tc *testing.T) {
		type T struct{ A map[interface{}]interface{} }
		_, err := c.NewRequest("GET", ".", &T{})
		assert.Error(tc, err, "should return an error")
	})

	t.Run("request with an invalid Method", func(tc *testing.T) {
		_, err := c.NewRequest("\n", "/", nil)
		assert.Error(tc, err, "should return an error")
	})

	t.Run("request with an empty body", func(tc *testing.T) {
		req, err := c.NewRequest("GET", ".", nil)
		assert.Nil(tc, err, "should not return an error")
		assert.Nil(tc, req.Body, "should return an empty body")
	})

}

// TestDo confirms that Do returns a JSON decoded value when making a request. It
// confirms the correct verb was used and that the decoded response value matches
// the expected result.
func TestDo(t *testing.T) {
	t.Run("successful GET request", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, "GET", r.Method)
			fmt.Fprint(w, `{"A":"a"}`)
		})

		want := &foo{"a"}
		got := new(foo)

		req, _ := client.NewRequest("GET", ".", nil)
		client.Do(context.Background(), req, got)

		assert.ObjectsAreEqual(want, got)
	})

	t.Run("GET request that returns an HTTP error", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		})

		req, _ := client.NewRequest("GET", ".", nil)
		resp, err := client.Do(context.Background(), req, nil)

		assert.Equal(tc, http.StatusInternalServerError, resp.StatusCode)
		assert.Error(tc, err, "should return an error")
	})

	t.Run("GET request that receives an empty payload", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
		})

		req, _ := client.NewRequest("GET", ".", nil)
		got := new(foo)
		resp, err := client.Do(context.Background(), req, got)

		assert.Equal(tc, http.StatusOK, resp.StatusCode)
		assert.Nil(tc, err, "should not return an error")
	})

	t.Run("GET request that receives an HTML response", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			html := `<!doctype html>
			<html lang="en-GB">
			<head>
			  <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
			  <title>Default Page Title</title>
			  <link rel="shortcut icon" href="favicon.ico">
			  <link rel="icon" href="favicon.ico">
			  <link rel="stylesheet" type="text/css" href="styles.css">
			</head>

			<body>

			</body>
			</html>	`
			fmt.Fprintln(w, html)
		})

		req, _ := client.NewRequest("GET", ".", nil)
		got := new(foo)
		resp, err := client.Do(context.Background(), req, got)

		assert.Equal(tc, http.StatusOK, resp.StatusCode)
		assert.Error(tc, err, "should return an error")
	})

	t.Run("request on a cancelled context", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
		})

		req, _ := client.NewRequest("GET", ".", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resp, err := client.Do(ctx, req, nil)

		assert.Error(tc, err, "should return an error")
		assert.Nil(tc, resp, "should not return a response")
	})

	t.Run("GET request that returns a forbidden response", func(tc *testing.T) {
		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(tc, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusForbidden)
		})

		req, _ := client.NewRequest("GET", ".", nil)
		resp, err := client.Do(context.Background(), req, nil)

		assert.Equal(tc, http.StatusForbidden, resp.StatusCode)
		assert.Error(tc, err, "should return an error")
		if _, ok := err.(AuthError); ok == false {
			t.Errorf("should return a starling.AuthError: %T", err)
		}
		if err, ok := err.(Error); ok == true && err.Temporary() == true {
			t.Errorf("should not return a temporary error")
		}

	})

}

// Setup establishes a test Server that can be used to provide mock responses during testing.
// It returns a pointer to a client, a mux, the server URL and a teardown function that
// must be called when testing is complete.
func setup() (client *Client, mux *http.ServeMux, serverURL string, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	c := NewClient(nil, "123456")
	url, _ := url.Parse(server.URL + "/")
	c.baseURL = url

	return c, mux, server.URL, server.Close
}
