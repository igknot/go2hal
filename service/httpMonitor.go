package service

import (
	"github.com/zamedic/go2hal/database"
	"time"
	"net/http"
	"fmt"
	"log"
	"strings"
	"errors"
	"bytes"
	"net/url"
	"io/ioutil"
	"gopkg.in/kyokomi/emoji.v1"
)

//HTTPMonitor is the current status of the monitor
type HTTPMonitor struct {
	running bool
}

var h *HTTPMonitor

func init() {
	h = &HTTPMonitor{}
	go func() {
		monitorEndpoints()
	}()
}

/*
CheckEndpoint checks the http endpoint to ensure it passes
 */
func CheckEndpoint(endpoint *database.HTTPEndpoint) error {
	response, err := doHTTPEndpoint(endpoint)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("response code recevied %d", response.StatusCode)
	}
	return nil
}

func monitorEndpoints() {
	log.Println("Starting HTTP Endpoint monitor")
	h.running = true
	for true {
		endpoints := database.GetHTMLEndpoints()
		if endpoints != nil {
			for _, endpoint := range endpoints {
				checkHTTP(&endpoint)
			}
		}
		time.Sleep(time.Minute * 2)
	}
}

func checkHTTP(endpoint *database.HTTPEndpoint) {
	response, err := doHTTPEndpoint(endpoint)
	if err != nil {
		s := emoji.Sprintf(":smoking: :x: *Smoke Test  Alert*\nName: %s \nEndpoint: %s \nError: %s", endpoint.Name,
			endpoint.Endpoint, err.Error())
		checkAlert(endpoint, s)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		msg, _ := ioutil.ReadAll(response.Body)
		error := fmt.Sprintf(":smoking: :x: *HTTP Alert*\nName: %s \nEndpoint: %s \nDid not receive a 200 success "+
			"response code. Recieved %d response code. Body Message %s", endpoint.Name, endpoint.Endpoint,
			response.StatusCode, msg)
		checkAlert(endpoint, error)
		return
	}
	if !endpoint.Passing && endpoint.ErrorCount >= endpoint.Threshold {
		SendAlert(emoji.Sprintf(":smoking: :white_check_mark: smoke test %s back to normal", endpoint.Name))
	}

	database.SuccessfulEndpointTest(endpoint.ID.String())
}

func checkAlert(endpoint *database.HTTPEndpoint, msg string) {
	database.FailedEndpointTest(endpoint, msg)
	if endpoint.Threshold > 0 && endpoint.Threshold == endpoint.ErrorCount {
		SendAlert(msg)
		InvokeCallout(fmt.Sprintf("Some Test failures for %s", endpoint.Name))
	}
}

func doHTTPEndpoint(endpoint *database.HTTPEndpoint) (*http.Response, error) {
	switch method := strings.ToUpper(endpoint.Method); method {
	case "POST":
		if len(endpoint.Parameters) > 1 {
			return nil, errors.New("POST expects 0 or 1 parameters to pass as a body, normally a json string. To send a form, please use method POST_FORM")
		}
		body := ""
		if len(endpoint.Parameters) == 1 {
			body = endpoint.Parameters[0].Value
		}
		return http.Post(endpoint.Endpoint, "application/json", bytes.NewBufferString(body))
	case "POST_FORM":
		values := url.Values{}
		for _, value := range endpoint.Parameters {
			values.Add(value.Name, value.Value)
		}
		return http.PostForm(endpoint.Endpoint, values)
	default:
		return http.Get(endpoint.Endpoint)
	}

}
