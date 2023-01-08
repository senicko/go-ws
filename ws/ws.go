package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrUpgradeFailed = errors.New("failed to upgrade to the WebSocket protocol")
)

// Upgrade upgrades the HTTP connection to use the WebSocket protocol.
func Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		handleError(w, nil, http.StatusMethodNotAllowed)
		return
	}

	if err := checkForHeader(r, "Connection", "Upgrade"); err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	if err := checkForHeader(r, "Upgrade", "websocket"); err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	if err := checkForHeader(r, "Sec-WebSocket-Version", "13"); err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		handleError(w, fmt.Errorf("'Sec-WebSocket-Key' missing: %w", ErrUpgradeFailed), http.StatusBadRequest)
		return
	}

	// FIXME: Probably we should validate more things
	// https://www.rfc-editor.org/rfc/rfc6455#section-4.1

	hj, ok := w.(http.Hijacker)
	if !ok {
		handleError(w, nil, http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	res := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept:" + generateAcceptKey(key) + "\r\nSec-WebSocket-Extensions: permessage-deflate; client_max_window_bits"
	if _, err := conn.Write([]byte(res)); err != nil {
		fmt.Println("failed to send the server handshake")
	}

	for {
	}
}

// generateAcceptKey generates a value for Sec-WebSocket-Accept header.
func generateAcceptKey(key string) string {
	// FIXME: It is stated that SHA-1 is cryptographically broken and shouldn't be used (?)
	// https://pkg.go.dev/crypto/sha1@go1.19.4
	h := sha1.New()
	io.WriteString(h, key+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// checkForHeader checks if the request headers contain a header with specific value.
func checkForHeader(r *http.Request, h, v string) error {
	if r.Header.Get(h) == v {
		return nil
	}

	return fmt.Errorf("'%s' is not set to '%s': %w", h, v, ErrUpgradeFailed)
}

// handleError sends and HTTP error to the client and logs debug message if present.
func handleError(w http.ResponseWriter, err error, s int) {
	if err != nil {
		fmt.Println(err)
	}

	http.Error(w, http.StatusText(s), s)
}
