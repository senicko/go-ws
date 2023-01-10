package ws

import (
	"fmt"
	"net"
)

// Conn represents a WebSocket connection.
type Conn struct {
	Conn net.Conn
}

// newConn creates a new WebSocket connection.
func newConn(conn net.Conn, readBufferSize int, writeBufferSize int) *Conn {
	return &Conn{
		Conn: conn,
	}
}

// Waits for incoming message from the client.
func (c *Conn) ReadMessage() string {
	buf := make([]byte, 1024)

	if _, err := c.Conn.Read(buf); err != nil {
		fmt.Println("failed to read incoming message")
		return ""
	}

	frame := decodeFrame(buf)
	return unmaskTextPayload(frame.maskingKey, frame.payload)
}

const (
	// Frame byte 0 bits
	bitFin  = 1 << 7
	bitRsv1 = 1 << 6
	bitRsv2 = 1 << 5
	bitRsv3 = 1 << 4

	// Frame byte 1 bits
	bitMask = 1 << 7
)

// frame is an internal struct which holds decoded frame information.
// Field names are the same as used in https://www.rfc-editor.org/rfc/rfc6455#section-5.2
type frame struct {
	fin           bool
	rsv1          bool
	rsv2          bool
	rsv3          bool
	opcode        byte
	mask          bool
	payloadLength uint8
	maskingKey    []byte
	payload       []byte
}

// Decodes incoming websocket frame.
func decodeFrame(buf []byte) *frame {
	// First byte
	fin := buf[0]&bitFin != 0
	rsv1 := buf[0]&bitRsv1 != 0
	rsv2 := buf[0]&bitRsv2 != 0
	rsv3 := buf[0]&bitRsv3 != 0
	opcode := buf[0] & 0xf

	// Second byte
	mask := buf[1]&bitMask != 0
	payloadLength := getPayloadLength(buf[1])

	var maskingKey []byte
	if mask {
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

func getPayloadLength(b byte) uint8 {
	return b & 0x7f
}

func unmaskTextPayload(mask, payload []byte) string {
	var transformed []byte
	for i, b := range payload {
		transformed = append(transformed, b^mask[i%4])
	}
	return string(transformed)
}
