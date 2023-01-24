package ws

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"testing"
)

// TODO: Test frame decoding
// TODO: Test getting payload length

// maskMessage is a utility for getting a masking key and the masked payload to use in tests..
func maskPayload(b []byte) ([]byte, []byte, error) {
	maskingKey := make([]byte, 4)
	if _, err := rand.Read(maskingKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate masking key: %w", err)
	}

	return applyMask(b, maskingKey), maskingKey, nil
}

func TestReadUnfragmentedTextMessage(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	ws := NewConn(server, 1024, 1024)
	defer ws.Close()

	payload := []byte("test")
	maskedPayload, maskingKey, err := maskPayload(payload)
	if err != nil {
		t.Errorf("failed to prepare masked payload")
	}

	frame := []byte{0x81, 0x84}
	frame = append(frame, maskingKey...)
	frame = append(frame, maskedPayload...)

	go func() {
		if _, err := client.Write(frame); err != nil {
			t.Error("failed to write to the WebSocket", err)
		}
	}()

	received, err := ws.ReadMessage()
	if err != nil {
		t.Error("failed to read the test message", err)
	}

	if !bytes.Equal(payload, received) {
		t.Errorf("expected %s got %s", payload, received)
	}
}

func TestReadFragmentedTextMessage(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	ws := NewConn(server, 1024, 1024)
	defer ws.Close()

	payload := []byte("test")
	payloadFragments := [][]byte{payload[:2], payload[2:]}

	go func() {
		for i, part := range payloadFragments {
			maskedPayload, maskingKey, err := maskPayload(part)
			if err != nil {
				t.Errorf("failed to prepare masked payload")
			}

			frame := []byte{0x1, 0x80}

			if i == len(payloadFragments)-1 {
				frame[0] |= bitFin
			}

			frame[1] |= uint8(len(part))
			frame = append(frame, maskingKey...)
			frame = append(frame, maskedPayload...)

			if _, err := client.Write(frame); err != nil {
				t.Error("failed to write to the WebSocket", err)
			}
		}
	}()

	received, err := ws.ReadMessage()
	if err != nil {
		t.Error("failed to read the test message: ", err)
	}

	if !bytes.Equal(payload, received) {
		t.Errorf("expected %s got %s", payload, received)
	}
}
