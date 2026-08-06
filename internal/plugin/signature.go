package plugin

import (
	"bytes"
	"encoding/binary"
)

var signedDocumentNames = []string{"README.md", "README.en.md", "LICENSE"}

// SignaturePayload binds user-visible package documents to the signed manifest.
// Artifact hashes are already part of the manifest itself.
func SignaturePayload(manifest []byte, files map[string][]byte) []byte {
	var payload bytes.Buffer
	payload.WriteString("meerkit-plugin-signature-v1")
	writeSignatureChunk(&payload, "meerkit-plugin.yaml", manifest)
	for _, name := range signedDocumentNames {
		if data, ok := files[name]; ok {
			writeSignatureChunk(&payload, name, data)
		}
	}
	return payload.Bytes()
}

func writeSignatureChunk(payload *bytes.Buffer, name string, data []byte) {
	_ = binary.Write(payload, binary.BigEndian, uint32(len(name)))
	payload.WriteString(name)
	_ = binary.Write(payload, binary.BigEndian, uint64(len(data)))
	payload.Write(data)
}
