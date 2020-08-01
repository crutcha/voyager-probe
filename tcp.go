package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"gopkg.in/guregu/null.v4"
	"net"
	"strconv"
	"strings"
	"time"
)

type TCPHeader struct {
	Source      uint16
	Destination uint16
	SeqNum      uint32
	AckNum      uint32
	DataOffset  uint8 // 4 bits
	Reserved    uint8 // 3 bits
	ECN         uint8 // 3 bits
	Ctrl        uint8 // 6 bits
	Window      uint16
	Checksum    uint16 // Kernel will set this if it's 0
	Urgent      uint16
	Options     []TCPOption
}

type TCPOption struct {
	Kind   uint8
	Length uint8
	Data   []byte
}

func (tcp *TCPHeader) Marshal() []byte {

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, tcp.Source)
	binary.Write(buf, binary.BigEndian, tcp.Destination)
	binary.Write(buf, binary.BigEndian, tcp.SeqNum)
	binary.Write(buf, binary.BigEndian, tcp.AckNum)

	var mix uint16
	mix = uint16(tcp.DataOffset)<<12 |
		uint16(tcp.Reserved)<<9 |
		uint16(tcp.ECN)<<6 |
		uint16(tcp.Ctrl)
	binary.Write(buf, binary.BigEndian, mix)

	binary.Write(buf, binary.BigEndian, tcp.Window)
	binary.Write(buf, binary.BigEndian, tcp.Checksum)
	binary.Write(buf, binary.BigEndian, tcp.Urgent)

	for _, option := range tcp.Options {
		binary.Write(buf, binary.BigEndian, option.Kind)
		if option.Length > 1 {
			binary.Write(buf, binary.BigEndian, option.Length)
			binary.Write(buf, binary.BigEndian, option.Data)

		}

	}

	out := buf.Bytes()

	// Pad to min tcp header size, which is 20 bytes (5 32-bit words)
	pad := 20 - len(out)
	for i := 0; i < pad; i++ {
		out = append(out, 0)

	}

	return out
}

func calcTCPChecksum(data []byte, srcip, dstip [4]byte) uint16 {

	pseudoHeader := []byte{
		srcip[0], srcip[1], srcip[2], srcip[3],
		dstip[0], dstip[1], dstip[2], dstip[3],
		0,                  // zero
		6,                  // protocol number (6 == TCP)
		0, byte(len(data)), // TCP length (16 bits), not inc pseudo header

	}

	sumThis := make([]byte, 0, len(pseudoHeader)+len(data))
	sumThis = append(sumThis, pseudoHeader...)
	sumThis = append(sumThis, data...)

	lenSumThis := len(sumThis)
	var nextWord uint16
	var sum uint32
	for i := 0; i+1 < lenSumThis; i += 2 {
		nextWord = uint16(sumThis[i])<<8 | uint16(sumThis[i+1])
		sum += uint32(nextWord)

	}
	if lenSumThis%2 != 0 {
		//fmt.Println("Odd byte")
		sum += uint32(sumThis[len(sumThis)-1])

	}

	// Add back any carry, and any carry from adding the carry
	sum = (sum >> 16) + (sum & 0xffff)
	sum = sum + (sum >> 16)

	// Bitwise complement
	return uint16(^sum)

}

func craftTCPSYNHeader(src, dst net.IP, srcPort, dstPort uint16) []byte {
	srcOctets := strings.Split(src.String(), ".")
	dstOctets := strings.Split(dst.String(), ".")

	// Need to convert IP type into byte array containing each octet
	// We dont need to bother with error checking since a valid net.IP struct will always
	// be able to be converted to int
	srcFirst, _ := strconv.Atoi(srcOctets[0])
	srcSecond, _ := strconv.Atoi(srcOctets[1])
	srcThird, _ := strconv.Atoi(srcOctets[2])
	srcFourth, _ := strconv.Atoi(srcOctets[3])
	srcBytes := [4]byte{byte(srcFirst), byte(srcSecond), byte(srcThird), byte(srcFourth)}

	dstFirst, _ := strconv.Atoi(dstOctets[0])
	dstSecond, _ := strconv.Atoi(dstOctets[1])
	dstThird, _ := strconv.Atoi(dstOctets[2])
	dstFourth, _ := strconv.Atoi(dstOctets[3])
	dstBytes := [4]byte{byte(dstFirst), byte(dstSecond), byte(dstThird), byte(dstFourth)}

	header := TCPHeader{
		Source:      srcPort,
		Destination: dstPort,
		SeqNum:      0,
		AckNum:      0,
		DataOffset:  5,   // 4 bits
		Reserved:    0,   // 3 bits
		ECN:         0,   // 3 bits
		Ctrl:        2,   // 6 bits (000010, SYN bit set)
		Window:      0x0, // size of your receive window
		Checksum:    0,   // we will calc and set this later
		Urgent:      0,
		Options:     []TCPOption{},
	}

	payload := header.Marshal()

	// Instead of doing 2 passes through Marshal, update byte array in palce
	checkSum := calcTCPChecksum(payload, srcBytes, dstBytes)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checkSum)
	payload[17], payload[18] = checksumBytes[0], checksumBytes[1]

	return payload
}

type TCPProbeExecutor struct {
	ProbeTarget
}

func (u *TCPProbeExecutor) Execute(target string, port, count int) ([]ProbeResponse, error) {
	log.Info("Starting TCP probes to ", target)

	currentTTL := 1
	reachedDest := false
	hops := make([]ProbeResponse, 0)
	for !reachedDest {
		for i := 0; i < count; i++ {
			rawConn, rawErr := net.Dial("ip4:tcp", target)
			if rawErr != nil {
				log.Warn("Error setting up raw socket for TCP: ", rawErr)
			}

			ip4Conn := ipv4.NewConn(rawConn)
			ip4Conn.SetTTL(currentTTL)

			srcIP := net.ParseIP(rawConn.LocalAddr().String())
			dstIP := net.ParseIP(target)
			payload := craftTCPSYNHeader(srcIP, dstIP, 32018, port)
			rawConn.Write(payload)
			reply := make([]byte, 1514)
			fmt.Println("written")

			fmt.Println("Wait for reply")
			rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			readBytes, readErr := rawConn.Read(reply)
			fmt.Println("Done waiting")

			sentTime := time.Now()
			_, writeErr := dialerConn.Write([]byte("test"))
			if writeErr != nil {
				panic(writeErr)
			}

			probeResponse := ProbeResponse{TTL: currentTTL}

			response, lookupErr := lookupResponses(target)
			if lookupErr != nil {
				log.Info(lookupErr)

				hops = append(hops, probeResponse)

				// using defer was leaking sockets but explicitly closing them is not
				dialerConn.Close()
				continue
			}

			dialerConn.Close()

			// FOR TESTING ONLY
			thisResponse := response[0]
			rtt := thisResponse.Timestamp.Sub(sentTime)
			probeResponse.IP = null.StringFrom(thisResponse.Source.String())
			probeResponse.Time = rtt.Milliseconds()
			probeResponse.HeaderSource = thisResponse.OriginalHeader.Src
			probeResponse.HeaderDest = thisResponse.OriginalHeader.Dst
			probeResponse.Responded = true

			hops = append(hops, probeResponse)
			if thisResponse.Response.Code == 3 && !reachedDest {
				log.Debug("Received type ", thisResponse.Response.Type, ". Stopping probes.")
				reachedDest = true
			}
		}
		currentTTL++
		if currentTTL == MAX_HOPS {
			log.Info("Max hops exceeded for probe to ", target)
			break
		}
		if reachedDest {
			log.Info("Probe complete: ", target)
		}
	}

	// TODO: error handling
	return hops, nil
}
