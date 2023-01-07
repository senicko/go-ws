package ws

import (
	"fmt"
	"net/http"
)

// checkForHeader checks if the request headers contain a header with specific value.
func checkForHeader(r *http.Request, h, v string) bool {
	return r.Header.Get(h) == v
}

// reportError sends and HTTP error to the client and logs debug message if present.
func reportError(w http.ResponseWriter, s int, m string) {
	if m != "" {
		fmt.Println(m)
	}

	http.Error(w, http.StatusText(s), s)
}

// Upgrade upgrades the HTTP connection to use the WebSocket protocol.
func Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		reportError(w, http.StatusMethodNotAllowed, "")
		return
	}

	if checkForHeader(r, "Connection", "upgrade") {
		reportError(w, http.StatusBadRequest, "upgrade failed, 'Connection' is not set to 'upgrade'")
		return
	}

	if checkForHeader(r, "Upgrade", "websocket") {
		reportError(w, http.StatusBadRequest, "upgrade failed, 'Upgrade' is not set to 'websocket'")
		return
	}

	// FIXME: Check how non-browser clients should be handled
	// https://www.rfc-editor.org/rfc/rfc6455#section-4.1
	if checkForHeader(r, "Origin", "") {
		reportError(w, http.StatusBadRequest, fmt.Sprintf("upgrade failed, 'Origin' is not valid: %s", r.Header.Get("Origin")))
		return
	}

	if checkForHeader(r, "Sec-WebSocket-Version", "13") {
		reportError(w, http.StatusBadRequest, "upgrade failed, 'Sec-WebSocket-Version' is not set to ''")
		return
	}

	key := w.Header().Get("Sec-WebSocket-Key")
	if key == "" {
		reportError(w, http.StatusBadRequest, "upgrade failed, 'Sec-WebSocket-Key' missing")
		return
	}

	// FIXME: Probably we should validate more things
	// https://www.rfc-editor.org/rfc/rfc6455#section-4.1

	hj, ok := w.(http.Hijacker)
	if !ok {
		reportError(w, http.StatusInternalServerError, "")
		return
	}

	conn, bufwr, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	bufwr.WriteString("Beep, Boop, I am TCP")
	bufwr.Flush()
}
