package ws

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	// Frame byte 0 bits
	bitFin  = 1 << 7
	bitRsv1 = 1 << 6
	bitRsv2 = 1 << 5
	bitRsv3 = 1 << 4

	// Frame byte 1 bits
	bitMask = 1 << 7
)

const (
	TextFrame            = 1
	BinaryFrame          = 2
	ConnectionCloseFrame = 8
	PingFrame            = 9
	PongFrame            = 10
)

// Conn represents a WebSocket connection.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

// newConn returns a new Conn.
func newConn(conn net.Conn, readBufferSize int, writeBufferSize int) *Conn {
	// TODO: use readBufferSize and writeBufferSize
	reader := bufio.NewReaderSize(conn, 1024)
	writer := bufio.NewWriterSize(conn, 1024)

	return &Conn{
		conn:   conn,
		reader: reader,
		writer: writer,
	}
}

// Close closes the websocket connection.
func (c *Conn) Close() {
	c.conn.Close()
}

// ReadMessage returns payload from inoming WebSocket frame.
func (c *Conn) ReadMessage() ([]byte, error) {
	frame, err := c.decodeFrame()
	fmt.Printf("%+v\n", frame)

	if err != nil {
		return nil, err
	}

	return c.unmaskTextPayload(frame.maskingKey, frame.payload), nil
}

// advanceBytes returns n next bytes from client's message.
func (c *Conn) advanceBytes(n uint64) ([]byte, error) {
	b := make([]byte, n)

	if _, err := io.ReadFull(c.reader, b); err != nil {
		return nil, err
	}

	return b, nil
}

// frame represents decoded WebSocket frame.
// https://www.rfc-editor.org/rfc/rfc6455#section-5.2
type frame struct {
	fin           bool
	rsv1          bool
	rsv2          bool
	rsv3          bool
	opcode        int
	mask          bool
	payloadLength uint64
	maskingKey    []byte
	payload       []byte
}

// decodeFrame returns an incoming WebSocket frame.
func (c *Conn) decodeFrame() (*frame, error) {
	b, err := c.advanceBytes(2)
	if err != nil {
		return nil, err
	}

	fin := b[0]&bitFin != 0
	rsv1 := b[0]&bitRsv1 != 0
	rsv2 := b[0]&bitRsv2 != 0
	rsv3 := b[0]&bitRsv3 != 0
	opcode := int(b[0] & 0xf)
	mask := b[1]&bitMask != 0

	payloadLength, err := c.getPayloadLength(b[1])
	if err != nil {
		return nil, err
	}

	var maskingKey []byte
	if mask {
		maskingKey, err = c.advanceBytes(4)
		if err != nil {
			return nil, err
		}
	}

	payload, err := c.advanceBytes(payloadLength)
	if err != nil {
		return nil, err
	}

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
	}, nil
}

// getPayloadLength returns the frame's payload length.
// b must be the second byte of WebSocket frame.
func (c *Conn) getPayloadLength(b byte) (uint64, error) {
	len := uint64(b & 0x7f)

	if len == 126 {
		b, err := c.advanceBytes(2)
		if err != nil {
			return 0, err
		}

		len = uint64(binary.BigEndian.Uint16(b))
	} else if len == 127 {
		b, err := c.advanceBytes(8)
		if err != nil {
			return 0, err
		}

		len = binary.BigEndian.Uint64(b)
	}

	return len, nil
}

// unmaskTextPayload unmasks the payload.
func (c *Conn) unmaskTextPayload(mask, payload []byte) []byte {
	var transformed []byte

	for i, b := range payload {
		transformed = append(transformed, b^mask[i%4])
	}

	return transformed
}

// SendMessage sends message to the client.
func (c *Conn) SendMessage(opcode int, data []byte) {
	b0 := byte(opcode)
	b0 |= bitFin

	var b1 byte
	if len(data) < 126 {
		b1 = byte(len(data))
	} else if len(data) < 0xFFFF {
		b1 = 126
	} else {
		b1 = 127
	}

	c.writer.Write([]byte{b0, b1})
	c.writer.Write(data)
	c.writer.Flush()
}
