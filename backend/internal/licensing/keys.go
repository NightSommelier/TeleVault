package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
)

const defaultKeyID = "tv-dev-2026-01"
const defaultPublicKeyBase64 = "K8nd9onZFQa4JAk0hvbRlKdWC5hZp/G7aozBq47TN90="

func DefaultPublicKeys() map[string]ed25519.PublicKey {
	keys := map[string]ed25519.PublicKey{}
	key, err := base64.StdEncoding.DecodeString(defaultPublicKeyBase64)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return keys
	}
	keys[defaultKeyID] = ed25519.PublicKey(key)
	return keys
}
