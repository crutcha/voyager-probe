package main

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestCalcTCPChecksum(t *testing.T) {
	assert := assert.New(t)

	fakePayload := []byte{
		0x92, 0x7e, 0x00, 0x50, 0xaf,
		0xc4, 0x8f, 0xa7, 0x00, 0x00,
		0x00, 0x00, 0xa0, 0x02, 0xfa,
		0xf0, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x04, 0x05, 0xb4, 0x04,
		0x02, 0x08, 0x0a, 0x20, 0x35,
		0xaa, 0x7b, 0x00, 0x00, 0x00,
		0x00, 0x01, 0x03, 0x03, 0x07,
	}

	checksum := calcTCPChecksum(fakePayload, [4]byte{192, 168, 10, 213}, [4]byte{172, 217, 4, 46})
	assert.Equal(uint16(0x339f), checksum, "checksum is valid")
}

func TestTCPHeaderMarshal(t *testing.T) {
	assert := assert.New(t)

	fakeHeader := TCPHeader{
		Source:      37502,
		Destination: 80,
		AckNum:      0,
		SeqNum:      2948894631,
		DataOffset:  10,
		Reserved:    0,
		ECN:         0,
		Ctrl:        2,
		Window:      64240,
		Checksum:    0,
		Urgent:      0,
	}

	fakePayload := []byte{
		0x92, 0x7e, 0x00, 0x50, 0xaf,
		0xc4, 0x8f, 0xa7, 0x00, 0x00,
		0x00, 0x00, 0xa0, 0x02, 0xfa,
		0xf0, 0x00, 0x00, 0x00, 0x00,
	}

	marshaledData := fakeHeader.Marshal()
	assert.Equal(fakePayload, marshaledData, "header marshaled correctly")
}

func TestCraftTCPSYNHeader(t *testing.T) {
	assert := assert.New(t)

	srcIP := net.ParseIP("192.168.10.213")
	dstIP := net.ParseIP("172.217.4.46")

	expectedPayload := []byte{
		0x92, 0x7e, 0x00, 0x50, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x50, 0x2, 0x00,
		0x00, 0x00, 0x8f, 0xa0, 0x00,
	}

	testPayload := craftTCPSYNHeader(srcIP, dstIP, uint16(37502), uint16(80))
	assert.Equal(expectedPayload, testPayload, "TCP SYN crafted accurately")
}
