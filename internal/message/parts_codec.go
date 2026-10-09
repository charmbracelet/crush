package message

import (
	"github.com/klauspost/compress/zstd"
)

// Stored parts are zstd-compressed to cut database size roughly 3x on
// long-lived setups, where parts JSON (conversation text, tool call
// payloads, base64 attachments) dominates the file. Decompression of a
// row is cheaper than the json.Unmarshal that follows it either way.
//
// Rows written before compression shipped store plain JSON. JSON parts
// always start with '[' (marshalParts wraps in an array), zstd frames
// start with a 4-byte magic number, so the two never collide and reads
// stay compatible with existing databases without a migration.

// zstdMagic is the first four bytes of every zstd frame: the format's
// fixed Magic_Number 0xFD2FB528 (RFC 8878 section 3.1.1.1), stored
// little-endian. Every conformant encoder emits it; the klauspost
// decoder validates the same constant (framedec.go frameMagic).
var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

// Both are safe for concurrent use: EncodeAll and DecodeAll operate on
// the calling goroutine and the underlying encoders/decoders are pooled
// internally by the library.
var (
	zstdEncoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	zstdDecoder, _ = zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
)

// compressParts compresses serialized parts for storage.
func compressParts(jsonParts []byte) []byte {
	return zstdEncoder.EncodeAll(jsonParts, nil)
}

// decompressPartsIfStored returns the stored parts as JSON, decompressing
// zstd frames and passing legacy plain-JSON rows through untouched.
func decompressPartsIfStored(data []byte) ([]byte, error) {
	if len(data) < len(zstdMagic) || string(data[:4]) != string(zstdMagic) {
		return data, nil
	}
	return zstdDecoder.DecodeAll(data, nil)
}

// DecodeStoredParts decodes a raw parts column value into content parts,
// handling both compressed and legacy plain-JSON rows. Exported for
// consumers that read the parts column directly (e.g. stats aggregation).
func DecodeStoredParts(stored []byte) ([]ContentPart, error) {
	jsonParts, err := decompressPartsIfStored(stored)
	if err != nil {
		return nil, err
	}
	return unmarshalParts(jsonParts)
}
