package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const pageSize int = 100

type DRFResponse struct {
	Count    uint   `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	//Results  []map[string][]ProbeTarget `json:"results"`
	Results []ProbeTarget `json:"results"`
}

type ProbeTarget struct {
	Destination string `json:"destination"`
	Interval    uint   `json:"interval"`
	ProbeCount  uint   `json:"probe_count"`
}

func getProbeTargets() ([]ProbeTarget, error) {
	client := &http.Client{Timeout: time.Second * 10}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/api/v1/probes/probe-targets/", voyagerServer), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", proberToken))

	q := url.Values{}
	q.Add("limit", strconv.Itoa(pageSize))

	targetArray := make([]ProbeTarget, 0)
	hasMoreResults := true
	currentOffset := 0
	for hasMoreResults == true {
		var payload DRFResponse
		q.Set("offset", strconv.Itoa(currentOffset))
		req.URL.RawQuery = q.Encode()

		resp, requestErr := client.Do(req)
		if requestErr != nil {
			log.Warn(requestErr)
			return nil, requestErr
		}

		body, bodyErr := ioutil.ReadAll(resp.Body)
		if bodyErr != nil {
			log.Warn(bodyErr)
			return nil, bodyErr
		}

		if resp.StatusCode != 200 {
			err := fmt.Errorf("%s %s", resp.Status, body)
			log.Warn(err)
			return nil, err
		}

		json.Unmarshal(body, &payload)
		for _, target := range payload.Results {
			targetArray = append(targetArray, target)
		}

		if payload.Next != "" {
			currentOffset += pageSize
		} else {
			hasMoreResults = false
		}
	}

	return targetArray, nil
}
