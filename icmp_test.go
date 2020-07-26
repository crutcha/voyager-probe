package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestIcmpCleanup(t *testing.T) {
	assert := assert.New(t)
	response1 := ICMPResponse{
		Timestamp: time.Now(),
	}
	responsemap := ResponseMap{responses: map[string][]ICMPResponse{
		"test-node": []ICMPResponse{response1},
	}}

	t.FailNow()
	// this loops forever, we'd have to change the function to be able to effectively
	// test it
	cleanupICMPResponses(&responsemap)
	assert.Equal(1, len(responsemap.responses), "response not cleaned up")
}
