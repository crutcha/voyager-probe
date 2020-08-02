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
	"sync"
	"time"
)

func NewTCPProbeExecutor(target ProbeTarget) ProbeExecutor {
	return &TCPProbeExecutor{target}
}

type TCPHeader struct {
	Source      uint16
	Destination uint16
	SeqNum      uint32
	AckNum      uint32
	DataOffset  uint8
	Reserved    uint8
	ECN         uint8
	Ctrl        uint8
	Window      uint16
	Checksum    uint16
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
	binary.LittleEndian.PutUint16(checksumBytes, checkSum) // Why is this little endian?
	payload[17], payload[18] = checksumBytes[0], checksumBytes[1]

	return payload
}

type TCPProbeExecutor struct {
	ProbeTarget
}

func (u *TCPProbeExecutor) Execute(target string, port uint16, count int) ([]ProbeResponse, error) {
	// LookupAddr will return IPs even if IPs are passed in. For domain name targets, we'll only return
	// the first result, at least for now
	addrResult, addrErr := net.LookupHost(target)
	if addrErr != nil {
		return nil, addrErr
	}
	log.Debug(fmt.Sprintf("Look result: %s -> %s", target, addrResult[0]))
	target = addrResult[0]

	log.Info("Starting TCP probes to ", target)

	currentTTL := 1
	hops := make([]ProbeResponse, 0)
	for currentTTL <= MAX_HOPS {
		var probewg sync.WaitGroup
		batch := ProbeBatch{hops: make([]ProbeResponse, 0, count)}
		probewg.Add(count)

		for i := 0; i < count; i++ {
			go sendTCPProbe(&probewg, &batch, target, port, currentTTL)
		}
		probewg.Wait()

		hops = append(hops, batch.hops...)
		currentTTL++
		if batch.IsFinal(target) {
			break
		}
	}

	// TODO: error handling
	log.Debug("probe complete: ", target)
	return hops, nil
}

func sendTCPProbe(wg *sync.WaitGroup, batch *ProbeBatch, target string, port uint16, ttl int) {
	// Setup a listener so OS binds a source port for us to use
	ipAddr, addrErr := net.ResolveTCPAddr("tcp4", "0.0.0.0:0")
	if addrErr != nil {
		log.Warn("IPAddr err: ", addrErr)
		return
	}

	rawListener, listenerErr := net.ListenTCP("tcp4", ipAddr)
	if listenerErr != nil {
		log.Warn("Error setting up TCP listener: ", listenerErr)
		return
	}
	defer rawListener.Close()

	// Use the port we got from bind in new socket towards target
	rawConn, rawErr := net.Dial("ip4:tcp", target)
	if rawErr != nil {
		log.Warn("Error creating socket towards target: ", rawErr)
		return
	}
	defer rawConn.Close()
	ip4Conn := ipv4.NewConn(rawConn)
	ip4Conn.SetTTL(ttl)

	sourcePortString := strings.Split(rawListener.Addr().String(), ":")[1]
	sourcePortInt, _ := strconv.Atoi(sourcePortString)
	srcIP := net.ParseIP(rawConn.LocalAddr().String())
	dstIP := net.ParseIP(target)
	payload := craftTCPSYNHeader(srcIP, dstIP, uint16(sourcePortInt), port)
	sentTime := time.Now()

	rawConn.Write(payload)
	reply := make([]byte, 1514)
	rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// We only care to see if there was an error or not, what's returned to us
	// do not matter. Any response at all means a TCP handshake is being attempted.
	probeResponse := ProbeResponse{TTL: ttl}
	_, readErr := rawConn.Read(reply)
	if readErr != nil {
		lookupKey := fmt.Sprintf("tcp:%s:%s:%d", sourcePortString, target, port)
		log.Debug("TCP LOOKUP KEY: ", lookupKey)
		response, lookupErr := lookupResponses(lookupKey)
		if lookupErr != nil {
			log.Debug(lookupErr)
			batch.Add(probeResponse)
			wg.Done()
			return
		}

		rtt := response.Timestamp.Sub(sentTime)
		probeResponse.IP = null.StringFrom(response.Source.String())
		probeResponse.Time = rtt.Milliseconds()
		probeResponse.HeaderSource = response.OriginalHeader.Src
		probeResponse.HeaderDest = response.OriginalHeader.Dst
		probeResponse.Responded = true
	} else {
		rtt := time.Now().Sub(sentTime)
		probeResponse.IP = null.StringFrom(target)
		probeResponse.Time = rtt.Milliseconds()
		probeResponse.Responded = true
	}

	batch.Add(probeResponse)
	wg.Done()
}
