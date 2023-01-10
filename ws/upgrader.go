package ws

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	ErrUpgradeFailed = errors.New("failed to upgrade to the WebSocket protocol")
)

type Upgrader struct {
	// CheckOrigin returns boolean indicating if the origin is acceptable by the server.
	CheckOrigin func(r *http.Request) bool
	Protocol    string

	// RW buffer sizes
	ReadBufferSize  int
	WriteBufferSize int
}

// Upgrade upgrades the HTTP connection to use the WebSocket protocol.
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	// Validate handshake

	if err, s := u.checkHandshake(r); err != nil {
		handleError(w, s)
		return nil, err
	}

	if u.CheckOrigin != nil && !u.CheckOrigin(r) {
		handleError(w, http.StatusForbidden)
		return nil, fmt.Errorf("client's origin validation failed: %w", ErrUpgradeFailed)
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		handleError(w, http.StatusBadRequest)
		return nil, fmt.Errorf("'Sec-WebSocket-Key' header missing: %w", ErrUpgradeFailed)
	}

	protocol := u.resolveSubprotocol(r)

	// FIXME: Probably we should validate more things
	// https://www.rfc-editor.org/rfc/rfc6455#section-4.1

	hj, ok := w.(http.Hijacker)
	if !ok {
		handleError(w, http.StatusInternalServerError)
		return nil, fmt.Errorf("ResponseWriter is not hijackable")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		handleError(w, http.StatusInternalServerError)
		return nil, err
	}

	if bufrw.Reader.Buffered() > 0 {
		conn.Close()
		return nil, fmt.Errorf("data send after opening handshake")
	}

	// Send response

	resHeaders := http.Header{}
	resHeaders.Set("Upgrade", "websocket")
	resHeaders.Set("Connection", "Upgrade")
	resHeaders.Set("Sec-WebSocket-Accept", u.generateAcceptKey(key))

	if protocol != "" {
		resHeaders.Set("Sec-WebSocket-Prococol", protocol)
	}

	res := http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: http.StatusSwitchingProtocols,
		Header:     resHeaders,
	}

	var resBuf bytes.Buffer

	if err := res.Write(&resBuf); err != nil {
		return nil, fmt.Errorf("failed to write to connection buffer: %w", err)
	}

	if _, err := conn.Write(resBuf.Bytes()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write to connection: %w", err)
	}

	return newConn(conn, u.ReadBufferSize, u.WriteBufferSize), nil
}

// resolveSubprotocol finds subprotocol that satisfies both server and the client.
func (u *Upgrader) resolveSubprotocol(r *http.Request) string {
	protocols := r.Header.Get("Sec-Websocket-Protocol")

	for _, p := range strings.Split(protocols, ",") {
		if p == u.Protocol {
			return p
		}
	}

	return ""
}

// checkHandshake checks if request independent headers meet handshake requirements.
func (u *Upgrader) checkHandshake(r *http.Request) (error, int) {
	if r.Method != http.MethodGet {
		return fmt.Errorf("handshake must be a GET request: %w", ErrUpgradeFailed), http.StatusMethodNotAllowed
	}

	if err := u.checkForHeader(r, "Connection", "Upgrade"); err != nil {
		return err, http.StatusBadRequest
	}

	if err := u.checkForHeader(r, "Upgrade", "websocket"); err != nil {
		return err, http.StatusBadRequest
	}

	if err := u.checkForHeader(r, "Sec-WebSocket-Version", "13"); err != nil {
		return err, http.StatusBadRequest
	}

	return nil, 0
}

// generateAcceptKey generates a value for Sec-WebSocket-Accept header.
func (u *Upgrader) generateAcceptKey(key string) string {
	// FIXME: It is stated that SHA-1 is cryptographically broken and shouldn't be used (?)
	// https://pkg.go.dev/crypto/sha1@go1.19.4
	h := sha1.New()
	io.WriteString(h, key+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// checkForHeader checks if the request headers contain a header with specific value.
func (u *Upgrader) checkForHeader(r *http.Request, h, v string) error {
	if r.Header.Get(h) == v {
		return nil
	}

	return fmt.Errorf("'%s' is not set to '%s': %w", h, v, ErrUpgradeFailed)
}

// handleError sends and HTTP error to the client and logs debug message if present.
func handleError(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}
