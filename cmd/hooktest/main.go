// Tool hooktest creates a simple client that sends a webhook event request
// to the specified URL for specified git reference name (or else `master`
// branch is taken by default).
//
// This is helpful while testing for webhook service. Currently implemented
// for generic webhook service.
//
// TODO(vrongmeal): implement mock webhook services for all providers.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	execute := hooktestCmd()
	if err := execute(); err != nil {
		fmt.Printf("cannot execute: %v\n", err)
		os.Exit(1)
	}
}

// hooktestCmd returns a new flagset for the hooktest tool.
func hooktestCmd() func() error {
	fs := flag.NewFlagSet("hooktest", flag.ExitOnError)

	var (
		hookType   string
		payloadURL string
		secret     string
		refName    string
	)

	fs.StringVar(&hookType, "t", "generic", "type of webhook client to start")
	fs.StringVar(&payloadURL, "u", "", "payload url to send the request to")
	fs.StringVar(&secret, "s", "", "secret to verify the origin of request")
	fs.StringVar(&refName, "r", "refs/heads/master", "git reference name to send in update")

	return func() error {
		if err := fs.Parse(os.Args[1:]); err != nil {
			return err
		}

		var (
			headers  http.Header
			jsonBody []byte
			err      error
		)

		switch hookType {
		case "generic":
			headers, jsonBody, err = genericHook(secret, refName)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid hook type: %s", hookType)
		}

		return sendRequest(payloadURL, headers, jsonBody)
	}
}

// genericHook returns the headers and json body for generic hook service.
func genericHook(secret, refName string) (headers http.Header, jsonBody []byte, _ error) {
	reqBody, err := json.Marshal(map[string]string{"ref": refName})
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected error while parsing JSON: %v", err)
	}

	return nil, reqBody, nil
}

// sendRequest sends the request to payload url with the given headers and body.
func sendRequest(payloadURL string, headers http.Header, jsonBody []byte) error {
	req, err := http.NewRequest(http.MethodPost, payloadURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("cannot create request: %v", err)
	}

	req.Header = headers
	if headers == nil {
		req.Header = http.Header{}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot post request: %v", err)
	}
	defer resp.Body.Close() // nolint:errcheck

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read body: %v", err)
	}

	fmt.Printf(`Status Code: %d
Headers:     %v
Body:        %s
`, resp.StatusCode, resp.Header, string(body))

	return nil
}
