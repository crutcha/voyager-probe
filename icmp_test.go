package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestIcmpCleanupNoAction(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now(),
	}
	responsemap := ResponseMap{responses: map[string][]ICMPResponse{
		"test-node": []ICMPResponse{response1},
	}}

	removeStaleICMPResponses(&responsemap)
	responses, ok := responsemap.responses["test-node"]

	assert.Equal(1, len(responses), "response not cleaned up")
	assert.Equal(true, ok, "test-node key exists")
}

func TestIcmpCleanupSingleEntryKeyExists(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now(),
	}
	response2 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-2) * time.Minute),
	}
	response3 := ICMPResponse{
		Timestamp: time.Now(),
		//Timestamp: time.Now().Add(time.Duration(-30) * time.Second),
	}
	responsemap := ResponseMap{responses: map[string][]ICMPResponse{
		"test-node": []ICMPResponse{response1, response2, response3},
	}}

	removeStaleICMPResponses(&responsemap)
	responses, ok := responsemap.responses["test-node"]

	assert.Equal(2, len(responses), "only 1 response removed, key still exists")
	assert.Equal(true, ok, "test-node key exists")
}

func TestIcmpCleanupRemoveKey(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-2) * time.Minute),
	}
	response2 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-2) * time.Minute),
	}
	responsemap := ResponseMap{responses: map[string][]ICMPResponse{
		"test-node": []ICMPResponse{response1, response2},
	}}

	removeStaleICMPResponses(&responsemap)
	_, ok := responsemap.responses["test-node"]

	assert.Equal(false, ok, "test-node key removed")
}
