package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// Layout of the decoded license bundle produced by the generator.
const (
	payloadLen   = 11 // 4-byte user hash + 2-byte SKU + 1-byte features + 4-byte expiry
	signatureLen = ed25519.SignatureSize
	bundleLen    = payloadLen + signatureLen
)

// LicenseDetails holds the fields recovered from a decoded license key. The
// username is not stored in the key, only a 4-byte hash of it, so it cannot be
// recovered directly; UserHash is exposed so it can be checked against a
// candidate username supplied on the command line.
type LicenseDetails struct {
	UserHash    []byte
	SkuID       uint16
	FeatureMask uint8
	Expiry      time.Time
}

func main() {
	keyFlag := flag.String("key", "", "License key to inspect")
	pubFlag := flag.String("pubkey", "", "Base64-encoded Ed25519 public key")
	userFlag := flag.String("user", "", "Optional username to verify against the key's embedded hash")
	flag.Parse()

	if *keyFlag == "" || *pubFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Load the public key supplied by the license issuer.
	pubKey, err := LoadPublicKey(*pubFlag)
	if err != nil {
		fmt.Printf("[-] Invalid public key: %v\n", err)
		os.Exit(1)
	}

	// Decode the key back into its raw payload and signature.
	payload, signature, err := DecodeLicenseKey(*keyFlag)
	if err != nil {
		fmt.Printf("[-] Malformed license key: %v\n", err)
		os.Exit(1)
	}

	// The signature is the tamper check: any change to the payload (SKU,
	// features, expiry or user hash) invalidates it against the issuer's key.
	if !ed25519.Verify(pubKey, payload, signature) {
		fmt.Println("[-] SIGNATURE INVALID - license key has been tampered with or was")
		fmt.Println("    not signed by the supplied public key.")
		os.Exit(2)
	}

	details := ParsePayload(payload)

	fmt.Println("[+] Signature valid - license key is authentic.")
	fmt.Println("=============================================================================")
	fmt.Printf("  SKU ID        : %d\n", details.SkuID)
	fmt.Printf("  Feature Mask  : %d (0b%08b)\n", details.FeatureMask, details.FeatureMask)
	fmt.Printf("  Expiry Date   : %s\n", details.Expiry.Format("2006-01-02"))
	fmt.Printf("  User Hash     : %x\n", details.UserHash)

	// If a username was supplied, confirm it matches the embedded hash.
	if *userFlag != "" {
		if HashUsername(*userFlag) == [4]byte(details.UserHash) {
			fmt.Printf("  Username      : %q matches embedded hash\n", *userFlag)
		} else {
			fmt.Printf("  Username      : %q does NOT match embedded hash\n", *userFlag)
		}
	}
	fmt.Println("=============================================================================")

	// Report expiry status against the current date.
	if time.Now().After(details.Expiry) {
		fmt.Printf("[!] License EXPIRED on %s\n", details.Expiry.Format("2006-01-02"))
		os.Exit(3)
	}
	fmt.Printf("[+] License valid - %d day(s) remaining.\n", int(time.Until(details.Expiry).Hours()/24))
}

// LoadPublicKey decodes a Base64 (standard encoding) string into an
// ed25519.PublicKey, returning an error if the value is not valid Base64 or is
// not the correct length for an Ed25519 public key.
func LoadPublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, fmt.Errorf("not valid base64: %v", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// DecodeLicenseKey reverses the generator's encoding: it strips the hyphen
// chunking, Base32-decodes (standard alphabet, no padding) the result, and
// splits the bundle into its 11-byte payload and 64-byte signature. It returns
// an error if the key is not valid Base32 or is the wrong length.
func DecodeLicenseKey(key string) (payload, signature []byte, err error) {
	stripped := strings.ReplaceAll(strings.TrimSpace(key), "-", "")

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	bundle, err := encoder.DecodeString(strings.ToUpper(stripped))
	if err != nil {
		return nil, nil, fmt.Errorf("not valid base32: %v", err)
	}
	if len(bundle) != bundleLen {
		return nil, nil, fmt.Errorf("expected %d bytes, got %d", bundleLen, len(bundle))
	}
	return bundle[:payloadLen], bundle[payloadLen:], nil
}

// ParsePayload unpacks the 11-byte payload into its constituent fields,
// mirroring the little-endian packing performed by the generator.
func ParsePayload(payload []byte) LicenseDetails {
	return LicenseDetails{
		UserHash:    payload[0:4],
		SkuID:       binary.LittleEndian.Uint16(payload[4:6]),
		FeatureMask: payload[6],
		Expiry:      time.Unix(int64(binary.LittleEndian.Uint32(payload[7:11])), 0).UTC(),
	}
}

// HashUsername reproduces the generator's username hashing: the first 4 bytes
// of the SHA-256 of the trimmed, lower-cased username.
func HashUsername(username string) [4]byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(username))))
	return [4]byte(sum[:4])
}
