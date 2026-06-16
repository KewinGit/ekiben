package ui

import (
	"bytes"
	"encoding/binary"
	"io"
)

// readAllDemux reads a docker log stream, stripping the 8-byte stream headers
// when present. If the stream is not multiplexed it returns the raw bytes.
func readAllDemux(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return raw, err
	}
	var out bytes.Buffer
	i := 0
	for i+8 <= len(raw) {
		// header: [STREAM_TYPE, 0,0,0, SIZE(4 big-endian)]
		st := raw[i]
		if st > 2 { // not a valid stream type -> assume not multiplexed
			return raw, nil
		}
		size := int(binary.BigEndian.Uint32(raw[i+4 : i+8]))
		i += 8
		if i+size > len(raw) {
			out.Write(raw[i:])
			break
		}
		out.Write(raw[i : i+size])
		i += size
	}
	if out.Len() == 0 {
		return raw, nil
	}
	return out.Bytes(), nil
}
