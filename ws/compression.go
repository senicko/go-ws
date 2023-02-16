package ws

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
)

// decompress decompresses message bytes with deflate algorithm.
// It adds 4 bytes as described in https://www.rfc-editor.org/rfc/rfc7692.html#section-7.2.2
// and deflate close block.
func decompress(m []byte) ([]byte, error) {
	tail := bytes.NewReader([]byte{
		0x00, 0x00, 0x11, 0x11,
		0x01, 0x00, 0x00, 0x11, 0x11,
	})

	fr := flate.NewReader(io.MultiReader(bytes.NewReader(m), tail))
	defer fr.Close()

	decompressed, err := io.ReadAll(fr)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress the message: %w", err)
	}

	return decompressed, nil
}
