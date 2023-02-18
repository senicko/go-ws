package ws

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"strings"
)

// TODO: This functionality could be implemented as Go's io.Reader and io.Writer

// decompress decompresses message bytes as described in
// https://www.rfc-editor.org/rfc/rfc7692.html#section-7.2.2
func decompress(m []byte) ([]byte, error) {
	const tail =
	// Add four bytes as specified in RFC
	"\x00\x00\xff\xff" +
		// Add final block to squelch unexpected EOF error from flate reader.
		"\x01\x00\x00\xff\xff"

	fr := flate.NewReader(io.MultiReader(bytes.NewReader(m), strings.NewReader(tail)))
	defer fr.Close()

	decompressed, err := io.ReadAll(fr)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress the message: %w", err)
	}

	return decompressed, nil
}

// compress compresses message bytes as described in
// https://www.rfc-editor.org/rfc/rfc7692#section-7.2.1
func compress(m []byte) ([]byte, error) {
	var compressed bytes.Buffer

	fw, _ := flate.NewWriter(&compressed, flate.BestCompression)

	if _, err := fw.Write(m); err != nil {
		return nil, fmt.Errorf("failed to write message bytes: %w", err)
	}

	if err := fw.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush the message: %w", err)
	}

	if err := fw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close the flate writer: %w", err)
	}

	compressedBytes := compressed.Bytes()
	return compressedBytes[:len(compressedBytes)-4], nil
}
