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
	responsemap := ResponseMap{responses: map[string]ICMPResponse{
		"test-node": response1,
	}}

	removeStaleICMPResponses(&responsemap)
	_, ok := responsemap.responses["test-node"]

	assert.Equal(true, ok, "test-node key exists")
}

func TestIcmpCleanupRemoveKey(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-2) * time.Minute),
	}
	response2 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-15) * time.Second),
	}
	responsemap := ResponseMap{responses: map[string]ICMPResponse{
		"test-1": response1,
		"test-2": response2,
	}}

	removeStaleICMPResponses(&responsemap)
	_, test1ok := responsemap.responses["test-1"]
	_, test2ok := responsemap.responses["test-2"]

	assert.Equal(false, test1ok, "test-1 key removed")
	assert.Equal(true, test2ok, "test-2 key was not removed")
}

func TestResponseLookup(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-2) * time.Minute),
	}
	response2 := ICMPResponse{
		Timestamp: time.Now().Add(time.Duration(-15) * time.Second),
	}
	received = ResponseMap{responses: map[string]ICMPResponse{
		"test-1": response1,
		"test-2": response2,
	}}

	value1, lookupErr1 := lookupResponses("test-1")
	_, lookupErr2 := lookupResponses("test-3")
	assert.Equal(value1, response1, "lookup failed")
	assert.Equal(nil, lookupErr1, "lookup err is nill")
	assert.EqualError(lookupErr2, "Response lookup timed out: test-3")

}
