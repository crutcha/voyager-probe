package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const pageSize int = 100

type DRFResponse struct {
	Count    uint          `json:"count"`
	Next     string        `json:"next"`
	Previous string        `json:"previous"`
	Results  []ProbeTarget `json:"results"`
}

type ProbeTarget struct {
	Destination string `json:"destination"`
	Interval    uint   `json:"interval"`
	ProbeCount  int    `json:"probe_count"`
	Type        string `json:"probe_type"`
	Port        uint16 `json:"port"`
}

func getProbeTargets() ([]ProbeTarget, error) {
	client := &http.Client{Timeout: time.Second * 10}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/api/v1/probes/probe-targets/", voyagerServer), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", proberToken))

	log.Infof("Target URI: %s\n", req.URL)

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
		log.Debugf("body: %s", string(body))
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

func emitProbeResults(probe Probe) {
	payload, jsonErr := json.Marshal(probe)
	if jsonErr != nil {
		log.Warn("Error creating probe result payload: ", jsonErr)
	}

	client := &http.Client{Timeout: time.Second * 10}
	req, _ := http.NewRequest("POST", fmt.Sprintf("https://%s/api/v1/probe-results/", voyagerServer), bytes.NewBuffer(payload))
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", proberToken))
	req.Header.Add("Content-Type", "application/json")

	resp, requestErr := client.Do(req)
	if requestErr != nil {
		log.Warn(requestErr)
		return
	}

	// yea it's kinda dirty but we only want the ID back so whatever
	jsonBody := make(map[string]interface{})
	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		log.Warn(fmt.Sprintf("POST of probe results failed: [HTTP%d] %s\n", resp.StatusCode, string(respBody)))
	} else {
		json.Unmarshal(respBody, &jsonBody)
		log.Info(fmt.Sprintf("Published probe result: %+v", jsonBody["id"]))
	}
}
