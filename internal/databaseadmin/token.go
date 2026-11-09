package databaseadmin

import "encoding/base64"

// decodeOpaqueToken accepts only the canonical unpadded base64url representation
// of a 256-bit opaque token. Strict decoding rejects non-zero unused trailing bits.
func decodeOpaqueToken(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, false
	}
	return raw, true
}
