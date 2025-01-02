/*
 TODO:
 - logging
*/

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

type packetType uint8

const (
	Reserved packetType = iota
	CONNECT
	CONNACK
	PUBLISH
	PUBACK
	PUBREC
	PUBREL
	PUBCOMP
	SUBSCRIBE
	SUBACK
	UNSUBSCRIBE
	UNSUBACK
	PINGREQ
	PINGRESP
	DISCONNECT
	AUTH
)

func (pt packetType) String() string {
	return [...]string{
		"Reserved",
		"CONNECT",
		"CONNACK",
		"PUBLISH",
		"PUBACK",
		"PUBREC",
		"PUBREL",
		"PUBCOMP",
		"SUBSCRIBE",
		"SUBACK",
		"UNSUBSCRIBE",
		"UNSUBACK",
		"PINGREQ",
		"PINGRESP",
		"DISCONNECT",
		"AUTH",
	}[pt]
}

type client struct {
	conn net.Conn
}
type packet struct {
	fh    fixedHeader
	props properties

	keepAlive uint16
	cid       string
	username  string
	password  []byte

	willProps   properties
	willTopic   string
	willPayload []byte
}
type fixedHeader struct {
	pt     packetType
	flags  byte
	remLen int
	len    int
}
type stringPair struct {
	key string
	val string
}
type properties struct {
	plFormatId      byte
	msgExpInterval  uint32
	contentType     string
	respTopic       string
	corrData        []byte
	subId           uint32
	sessExpInterval uint32
	assignedClId    string
	serverKA        uint16
	authMethod      string
	authData        []byte
	reqProbInfo     byte
	willDelInterval uint32
	reqRespInfo     byte
	respInfo        string
	serverRef       string
	reasonStr       string
	recvMax         uint16
	topicAliasMax   uint16
	topicAlias      uint16
	maxQOS          byte
	retAvail        byte
	userProp        stringPair
	maxPacketSize   uint32
	wildSubAvail    byte
	subIdAvail      byte
	sharedSubAvail  byte
}

const connectTimeout = time.Second * 5
const defaultPacketSize = 1024

func addConn(conn net.Conn) {
	packetBuff := make([]byte, defaultPacketSize)

	err := conn.SetReadDeadline(time.Now().Add(connectTimeout))
	if err != nil {
		fmt.Printf("error setting read deadline: %v\n", err)
		return
	}

	n, err := conn.Read(packetBuff)
	if err != nil {
		fmt.Printf("error reading packet: %v\n", err)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			conn.Close()
		}
		return
	}

	p, err := decodePacket(packetBuff[0:n])
	if err != nil {
		fmt.Printf("error decoding packet: %v\n", err)
		return
	}
	if p.fh.pt != CONNECT {
		fmt.Printf("client attempted to send packet other than CONNECT first: %v\n", p.fh.pt)
		return
	}
}

// TODO:
// - deal with data not being big enough for whole packet
func decodePacket(data []byte) (packet, error) {
	var p packet

	fh, err := decodeFixedHeader(data)
	if err != nil {
		return p, err
	}
	p.fh = fh

	rest := data[fh.len : fh.len+fh.remLen]
	switch fh.pt {
	case CONNECT:
		fmt.Printf("connect packet\n")
		// check protocol name
		if string(rest[2:6]) != "MQTT" {
			return p, errors.New("invalid protocol name")
		}
		// check protocol version
		// TODO: support older versions
		if int(rest[6]) != 5 {
			return p, errors.New("invalid protocol version")
		}

		flags := rest[7]
		// check reserved flag
		if flags&1 != 0 {
			return p, errors.New("reserved flag set, malformed packet")
		}

		p.keepAlive = binary.BigEndian.Uint16(rest[8:10])

		propLen, propLenSize, err := decodeVariableByteInt(rest[10:])
		if err != nil {
			return p, err
		}
		props, err := decodeProps(rest[10+propLenSize : 10+propLenSize+int(propLen)])
		if err != nil {
			return p, err
		}
		p.props = props

		payload := rest[10+propLenSize+int(propLen):]

		l := binary.BigEndian.Uint16(payload[0:2])
		p.cid = string(payload[2 : 2+l])
		payload = payload[2+l:]

		if flags&0b00000100 != 0 {
			// will props
			willPropsLen, willPropsLenSize, err := decodeVariableByteInt(payload)
			if err != nil {
				return p, err
			}
			willProps, err := decodeProps(payload[willPropsLenSize : willPropsLenSize+int(willPropsLen)])
			if err != nil {
				return p, err
			}
			p.willProps = willProps
			payload = payload[willPropsLenSize+int(willPropsLen):]
			l := binary.BigEndian.Uint16(payload[0:2])
			p.willTopic = string(payload[2 : 2+int(l)])
			payload = payload[2+int(l):]
			l = binary.BigEndian.Uint16(payload[0:2])
			p.willPayload = payload[2 : 2+int(l)]
			payload = payload[2+int(l):]
		}

		if flags&0b10000000 != 0 {
			// user name
			l := binary.BigEndian.Uint16(payload[0:2])
			p.username = string(payload[2 : 2+int(l)])
			payload = payload[2+int(l):]
		}

		if flags&0b01000000 != 0 {
			l := binary.BigEndian.Uint16(payload[0:2])
			p.password = payload[2 : 2+int(l)]
		}
	default:
		return p, errors.New("unsupported packet type")
	}

	return p, nil
}

func decodeFixedHeader(data []byte) (fixedHeader, error) {
	var fh fixedHeader

	fh.pt = packetType(uint8(data[0] >> 4))
	fh.flags = 0b00001111 & data[0]
	remLen, remLenSize, err := decodeVariableByteInt(data[1:])
	if err != nil {
		return fh, err
	}

	fh.remLen = int(remLen)
	fh.len = remLenSize + 1

	return fh, nil
}

func decodeVariableByteInt(data []byte) (uint32, int, error) {
	var value uint32 = 0
	var multiplier uint32 = 1

	i := 0
	encodedByte := data[i]
	cont := true
	for cont {
		value += uint32(encodedByte&127) * multiplier
		if multiplier > 128*128*128 {
			return value, i + 1, errors.New("invalid variable byte integer")
		}
		multiplier *= 128
		cont = (encodedByte & 128) != 0
		if cont {
			i++
			encodedByte = data[i]
		}
	}

	return value, i + 1, nil
}

// TODO: maybe some more error handling
func decodeProps(data []byte) (properties, error) {
	var props properties

	i := 0
	for i < len(data) {
		id, l, err := decodeVariableByteInt(data[i:])
		if err != nil {
			return props, err
		}
		i += l
		switch id {
		case 0x01:
			// payload format indicator
			props.plFormatId = data[i]
			i += 1
		case 0x02:
			// message expiry interval
			props.msgExpInterval = binary.BigEndian.Uint32(data[i : i+4])
			i += 4
		case 0x03:
			// content type
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.contentType = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x08:
			// response topic
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.respTopic = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x09:
			// correlation data
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.corrData = data[i+2 : i+2+int(l)]
			i += 2 + int(l)
		case 0x0B:
			// subscription identifier
			subId, l, err := decodeVariableByteInt(data[i:])
			if err != nil {
				return props, err
			}
			props.subId = subId
			i += l
		case 0x11:
			// session expiry interval
			props.sessExpInterval = binary.BigEndian.Uint32(data[i : i+4])
			i += 4
		case 0x12:
			// assigned client identifier
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.assignedClId = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x13:
			// server keep alive
			props.serverKA = binary.BigEndian.Uint16(data[i : i+2])
			i += 2
		case 0x15:
			// authentication method
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.authMethod = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x16:
			// authentication data
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.authData = data[i+2 : i+2+int(l)]
			i += 2 + int(l)
		case 0x17:
			// request problem information
			props.reqProbInfo = data[i]
			i += 1
		case 0x18:
			// will delay interval
			props.willDelInterval = binary.BigEndian.Uint32(data[i : i+4])
			i += 4
		case 0x19:
			// request response information
			props.reqRespInfo = data[i]
			i += 1
		case 0x1A:
			// response information
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.respInfo = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x1C:
			// server reference
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.serverRef = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x1F:
			// reason string
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.reasonStr = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x21:
			// receive maximum
			props.recvMax = binary.BigEndian.Uint16(data[i : i+2])
			i += 2
		case 0x22:
			// topic alias maximum
			props.topicAliasMax = binary.BigEndian.Uint16(data[i : i+2])
			i += 2
		case 0x23:
			// topic alias
			props.topicAlias = binary.BigEndian.Uint16(data[i : i+2])
			i += 2
		case 0x24:
			// maximum qos
			props.maxQOS = data[i]
			i += 1
		case 0x25:
			// retain available
			props.retAvail = data[i]
			i += 1
		case 0x26:
			// user property

			// key
			l := binary.BigEndian.Uint16(data[i : i+2])
			props.userProp.key = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)

			// val
			l = binary.BigEndian.Uint16(data[i : i+2])
			props.userProp.val = string(data[i+2 : i+2+int(l)])
			i += 2 + int(l)
		case 0x27:
			// maximum packet size
			props.maxPacketSize = binary.BigEndian.Uint32(data[i : i+4])
			i += 4
		case 0x28:
			// wildcard subscription available
			props.wildSubAvail = data[i]
			i += 1
		case 0x29:
			// subscription identifier available
			props.subIdAvail = data[i]
			i += 1
		case 0x2A:
			// shared subscription available
			props.sharedSubAvail = data[i]
			i += 1
		default:
			return props, errors.New("invalid property type")
		}
	}

	return props, nil
}

func main() {
	listener, err := net.Listen("tcp", ":1883")
	if err != nil {
		fmt.Printf("error creating listener: %v\n", err)
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("error accepting connection: %v", err)
			continue
		}
		go addConn(conn)
	}
}
