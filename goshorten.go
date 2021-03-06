//
// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// goshorten is a demo application for App Engine Service Accounts. It
// shows how to use `appengine.AccessToken` method in combination with
// the URL shortener API.
package goshorten

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"appengine"
)

// appengineHandler wraps http.Handler to pass it a new `appengine.Context` and handle errors.
type appengineHandler func(c appengine.Context, w http.ResponseWriter, r *http.Request) error

func (h appengineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if err := h(c, w, r); err != nil {
		c.Errorf("%v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func init() {
	http.Handle("/", appengineHandler(handle))
	http.Handle("/shorten", appengineHandler(shorten))
}

// history maps to the JSON body returned by URL shortener API `history` method.
type historyResponse struct {
	Items []struct {
		Id      string
		LongUrl string
	}
	Error errorValue
}

// request maps to the JSON payload of `shorten` requests to the URL shortener API.
type shortenRequest struct {
	LongUrl string `json:"longUrl"`
}

// errorValue maps to the JSON body of error value in URL Shortener API responses.
type errorValue struct {
	Errors []struct {
		Reason, Message, Location string
	}
	Code    int
	Message string
}

// shortenResponse maps to the JSON body of `shorten` requests to the URL shortener API.
type shortenResponse struct {
	Error errorValue
}

var mainTemplate = template.Must(template.New("main").Parse(`<html>
<body>
  <h1>Go/App Engine Service account demo</h1>
  <form action="/shorten" method="POST">
    <label for="url">Enter URL:</url>
    <input type="text" name="url">
    <input type="submit" value="shorten!">
  </form>
  <h2>URLs recently shortened:</h2>
  <ul>
    {{range .Items}}
    <li>
      <a href="{{.Id}}" title="{{.Id}}">{{.LongUrl}}</a>
    </li>
    {{end}}
  </ul>
</body></html>`))

// handle renders the main page template with a submission form and the history of shortened urls.
func handle(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	client, err := authorizedClient(c, "https://www.googleapis.com/auth/urlshortener")
	if err != nil {
		return fmt.Errorf("error creating authorized client: %v", err)
	}
	resp, err := client.Get("https://www.googleapis.com/urlshortener/v1/url/history")
	if err != nil {
		return fmt.Errorf("error getting history: %v", err)
	}
	var result historyResponse
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("error decoding json body: %v", err)
	}
	c.Infof("urlshortener API response: %v", result)
	if result.Error.Code != 0 {
		return fmt.Errorf("urlshortener API error: %v", result.Error)
	}
	if err := mainTemplate.Execute(w, result); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}
	return nil
}

// shorten shortens a new url and redirects to the main page
func shorten(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	client, err := authorizedClient(c, "https://www.googleapis.com/auth/urlshortener")
	if err != nil {
		return fmt.Errorf("error creating authorized client: %v", err)
	}
	body, err := json.Marshal(&shortenRequest{LongUrl: r.FormValue("url")})
	if err != nil {
		return fmt.Errorf("error encoding JSON body: %v", err)
	}
	var result shortenResponse
	resp, err := client.Post("https://www.googleapis.com/urlshortener/v1/url", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error posting url: %v", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("error decoding json body: %v", err)
	}
	c.Infof("urlshortener API response: %v", result)
	if result.Error.Code != 0 {
		return fmt.Errorf("urlshortener API error: %v", result.Error)
	}
	http.Redirect(w, r, "/", 303)
	return nil
}
