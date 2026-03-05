package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
)

func cmdKeygen(_ []string, stdout, stderr io.Writer) int {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(stderr, "generate key: %v\n", err)
		return 1
	}

	// Output private key seed as base64 (32 bytes)
	seed := priv.Seed()
	fmt.Fprintf(stdout, "EVIDRA_SIGNING_KEY=%s\n\n", base64.StdEncoding.EncodeToString(seed))

	// Output public key as PEM
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fmt.Fprintf(stderr, "marshal public key: %v\n", err)
		return 1
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})
	fmt.Fprintf(stdout, "%s", pemBlock)

	return 0
}
