package ws

import (
	"fmt"
	"net"
)

type Conn struct {
	Conn net.Conn
}

// newConn creates a new WebSocket connection
func newConn(conn net.Conn, readBufferSize int, writeBufferSize int) *Conn {
	return &Conn{
		Conn: conn,
	}
}

// Waits for incoming message from the client
func (c *Conn) Message() string {
	buf := make([]byte, 1024)

	if _, err := c.Conn.Read(buf); err != nil {
		fmt.Println("failed to read incoming message")
		return ""
	}

	frame := decodeFrame(buf)
	return unmaskTextPayload(frame.maskingKey, frame.payload)
}

// frame is an internal struct which holds decoded frame information.
// Field names are the same as used in https://www.rfc-editor.org/rfc/rfc6455#section-5.2
type frame struct {
	fin           uint8
	rsv1          uint8
	rsv2          uint8
	rsv3          uint8
	opcode        uint8
	mask          uint8
	payloadLength uint8
	maskingKey    []byte
	payload       []byte
}

// Decodes incoming websocket frame
func decodeFrame(buf []byte) *frame {
	fin := buf[0] >> 7 & 1
	rsv1 := buf[0] >> 6 & 1
	rsv2 := buf[0] >> 5 & 1
	rsv3 := buf[0] >> 4 & 1
	opcode := buf[0] & 0xf
	mask := buf[1] >> 7 & 1
	payloadLength := getPayloadLength(buf[1])

	var maskingKey []byte
	if mask == 1 {
		maskingKey = buf[2:6]
	}

	payload := buf[6 : 6+payloadLength]

	return &frame{
		fin:           fin,
		rsv1:          rsv1,
		rsv2:          rsv2,
		rsv3:          rsv3,
		opcode:        opcode,
		mask:          mask,
		payloadLength: payloadLength,
		maskingKey:    maskingKey,
		payload:       payload,
	}
}

func unmaskTextPayload(mask, payload []byte) string {
	var transformed []byte
	for i, b := range payload {
		transformed = append(transformed, b^mask[i%4])
	}
	return string(transformed)
}

func getPayloadLength(b byte) uint8 {
	return b & 0x7f
}
