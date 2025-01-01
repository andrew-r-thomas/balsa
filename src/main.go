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

const connectTimeout = time.Second * 5
const defaultPacketSize = 1024

func handleConn(conn net.Conn) {
	packetBuff := make([]byte, defaultPacketSize)

	conn.SetReadDeadline(time.Now().Add(connectTimeout))
	n, err := conn.Read(packetBuff)
	if err != nil {
		fmt.Printf("error reading packet: %v\n", err)
		return
	}

	fh, err := decodeFixedHeader(packetBuff[0:n])
	if err != nil {
		fmt.Printf("error decoding fixed header: %v\n", err)
		return
	}
	if fh.pt != CONNECT {
		fmt.Printf("client attempted to send packet other than CONNECT first: %v\n", fh.pt)
		return
	}

	if fh.len+fh.remLen > n {
		// TODO: read more into the slice
	}

	rest := packetBuff[fh.len : fh.len+fh.remLen]
	// check protocol name
	if string(rest[2:6]) != "MQTT" {
		fmt.Printf("invalid protocol name\n")
		return
	}
	// check protocol version
	// TODO: support older versions
	if int(rest[6]) != 5 {
		fmt.Printf("invalid protocol version: %d\n", int(rest[6]))
		return
	}

	flags := rest[7]
	// check reserved flag
	if flags&1 != 0 {
		fmt.Printf("reserved flag set, malformed packet\n")
		return
	}

	keepAlive := binary.BigEndian.Uint16(rest[8:10])
	fmt.Printf("keep alive: %d\n", keepAlive)

	propLen, propLenSize, err := decodeVariableByteInt(rest[10:])
	if err != nil {
		fmt.Printf("error decoding prop len: %v\n", err)
		return
	}
	i := 0
	// TODO:
	// var sessionExpInterval uint32
	// var recvMax uint16
	// var maxPacketSize uint32
	// var topicAliasMax uint16
	// var reqRespInfo byte
	// var reqProbInfo byte
	// var userProp stringPair
	// var authMethod string
	// var authData string
	for i < int(propLen) {
		switch rest[10+i] {
		case 0x11:
			// session expiry interval
		case 0x21:
			// receive maximum
		case 0x27:
			// maximum packet size
		case 0x28:
			// topic alias maximum
		case 0x19:
			// request response information
		case 0x17:
			// request problem information
		case 0x26:
			// user property
		case 0x15:
			// authentication method
		case 0x16:
			// authentication data

		default:
			fmt.Printf("invalid property type\n")
			return
		}
	}

	rest = rest[10+propLenSize+int(propLen):]
	cidLen := binary.BigEndian.Uint16(rest[0:2])
	cid := string(rest[2 : 2+cidLen])
	fmt.Printf("client id: %s\n", cid)

	// TODO:
	if flags&0b00000100 != 0 {
		fmt.Printf("we got will props/topic/payload!\n")
	}
	if flags&0b10000000 != 0 {
		fmt.Printf("we got a user name!\n")
	}
	if flags&0b01000000 != 0 {
		fmt.Printf("we got a password!\n")
	}

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
		go handleConn(conn)
	}
}
