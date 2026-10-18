package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// zstd encoder/decoder are safe for concurrent use and are reused as singletons
// to avoid the cost of rebuilding them for every message.
var (
	zstdEncoderOnce sync.Once
	zstdEncoder     *zstd.Encoder
	zstdDecoderOnce sync.Once
	zstdDecoder     *zstd.Decoder
)

func rawJSONEncoder() *zstd.Encoder {
	zstdEncoderOnce.Do(func() {
		// Uploads are low-frequency writes, so we favor compression ratio.
		enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
		if err != nil {
			panic(err)
		}
		zstdEncoder = enc
	})
	return zstdEncoder
}

func rawJSONDecoder() *zstd.Decoder {
	zstdDecoderOnce.Do(func() {
		dec, err := zstd.NewReader(nil)
		if err != nil {
			panic(err)
		}
		zstdDecoder = dec
	})
	return zstdDecoder
}

// CompressRawJSON zstd-compresses a raw_json payload. An empty input yields a
// nil blob so empty messages stay empty on disk.
func CompressRawJSON(s string) []byte {
	if s == "" {
		return nil
	}
	return rawJSONEncoder().EncodeAll([]byte(s), make([]byte, 0, len(s)/4))
}

// DecompressRawJSON restores the original raw_json payload from a zstd blob.
func DecompressRawJSON(blob []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	decoded, err := rawJSONDecoder().DecodeAll(blob, nil)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// HashRawJSON returns a stable SHA-256 hex digest of a raw_json payload. It lets
// the upload diff prove a message is unchanged without zstd-decompressing the
// stored blob; hashing the incoming string is far cheaper than decompression.
// An empty payload still yields a (non-empty) digest, so a populated ContentHash
// always means the hash has been computed.
func HashRawJSON(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SetRawJSON compresses the given raw_json payload into RawZstd and records its
// content hash. RawZstd and ContentHash are always derived from the same input
// here, so they never drift; this is the only place that writes either field.
func (m *ThreadMessage) SetRawJSON(s string) {
	m.RawZstd = CompressRawJSON(s)
	m.ContentHash = HashRawJSON(s)
}

// RawJSON decompresses RawZstd back into the original raw_json payload.
func (m *ThreadMessage) RawJSON() (string, error) {
	return DecompressRawJSON(m.RawZstd)
}
