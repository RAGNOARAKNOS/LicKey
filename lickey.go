package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const PrivateKeyEnvVarName = "lickey_privatekey"

type LicenseData struct {
	Username    string
	SkuID       uint16
	FeatureMask uint8
	ExpiryDate  string // Format: YYYY-MM-DD
}

func main() {
	// Grab commandline args
	userFlag := flag.String("user", "", "Username")
	skuFlag := flag.Uint("sku", 0, "Product SKU ID")
	featFlag := flag.Uint("features", 0, "Feature Mask (0-255)")
	expiryFlag := flag.String("expiry", "", "Expiration (YYYY-MM-DD)")
	flag.Parse()

	// Fail if commandline args cant be parsed
	if *userFlag == "" || *skuFlag == 0 || *expiryFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Assign to License struct
	lic := LicenseData{
		Username:    *userFlag,
		SkuID:       uint16(*skuFlag),
		FeatureMask: uint8(*featFlag),
		ExpiryDate:  *expiryFlag,
	}

	// Try and load the private key
	privKey, err := LoadPrivateKeyFromEnv()

	// If no private key is found generate a new keypair
	if err != nil {
		fmt.Print(err)
		fmt.Printf("[!] Local environment variable '%s' not found or invalid.\n", PrivateKeyEnvVarName)
		fmt.Println("[*] Generating a fresh keypair for your deployment setup...")

		newPrivB64, newPubB64, keyErr := GenerateAndBase64EncodeKey()
		if keyErr != nil {
			log.Fatalf("Critical error generating keys: %v", keyErr)
		}

		fmt.Println("\n=============================================================================")
		fmt.Printf("1. SET THIS IN YOUR ENVIRONMENT VARIABLE OR .env FILE:\n\n")
		fmt.Printf("   %s %s\n\n", PrivateKeyEnvVarName, newPrivB64)
		fmt.Println("2. COPY THIS PUBLIC KEY TO YOUR APP CLIENT:")
		fmt.Printf("   %s\n", newPubB64)
		fmt.Println("=============================================================================")
		fmt.Println("\n[!] Please store these keys securely!!!")
		fmt.Println("\n[!] Please configure the environment variable and re-run the script.")
		return
	}

	//Construct the License Key
	lickey, err := GenerateLicenseKey(lic, privKey)
	if err != nil {
		log.Fatal("[-] Error: %v", err)
	}

	// Print the shiny new key
	//fmt.Printf("\n%s,%s,%d,%d,%s", lic.Username, lic.ExpiryDate, lic.SkuID, lic.FeatureMask, lickey)
	fmt.Printf("%s", lickey)
}

// GenerateLicenseKey builds a signed, human-readable license key from the
// supplied LicenseData and Ed25519 private key.
//
// It constructs an 11-byte payload consisting of a 4-byte SHA-256 hash of the
// normalised (trimmed, lower-cased) username, followed by the SkuID,
// FeatureMask, and the expiry date encoded as a little-endian uint32 Unix
// timestamp. The payload is signed with privKey, and the payload and 64-byte
// signature are concatenated, Base32-encoded (standard alphabet, no padding),
// and split into hyphen-separated five-character groups.
//
// The ExpiryDate field of lic must be in "2006-01-02" (YYYY-MM-DD) format. It
// returns the formatted license key, or an error if the expiry date cannot be
// parsed.
func GenerateLicenseKey(lic LicenseData, privKey ed25519.PrivateKey) (string, error) {
	// Parse Expiry
	time, err := time.Parse("2006-01-02", lic.ExpiryDate)
	if err != nil {
		return "", fmt.Errorf("invalid date format: %v", err)
	}
	expiry := uint32(time.Unix())

	// Hash the username
	hasher := sha256.New()
	hasher.Write([]byte(strings.TrimSpace(strings.ToLower(lic.Username))))
	userHash := hasher.Sum(nil)[:4]

	// Pack the data (11 bytes payload)
	buf := new(bytes.Buffer)
	buf.Write(userHash)
	_ = binary.Write(buf, binary.LittleEndian, lic.SkuID)
	_ = binary.Write(buf, binary.LittleEndian, lic.FeatureMask)
	_ = binary.Write(buf, binary.LittleEndian, expiry)
	payload := buf.Bytes()

	// Sign the data
	signature := ed25519.Sign(privKey, payload)

	// Combine the payload and signature
	finalBundle := append(payload, signature...)

	// Serialise into 120 characters (Base32)
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	b32String := encoder.EncodeToString(finalBundle)

	// Chunk into 5 letter segments
	var chunked []string
	for i := 0; i < len(b32String); i += 5 {
		chunked = append(chunked, b32String[i:i+5])
	}

	return strings.Join(chunked, "-"), nil
}

// GenerateAndBase64EncodeKey creates a fresh Ed25519 keypair using a
// cryptographically secure random source and returns both keys as Base64
// (standard encoding) text strings.
//
// It returns the private key and public key (in that order), or an error if
// key generation fails. The private key string is intended to be stored in the
// signing environment (e.g. the lickey_privatekey variable), while the public
// key string is distributed to clients so they can verify license signatures.
func GenerateAndBase64EncodeKey() (string, string, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Encode both keys to Base64 standard text strings
	privBase64 := base64.StdEncoding.EncodeToString(privKey)
	pubBase64 := base64.StdEncoding.EncodeToString(pubKey)

	return privBase64, pubBase64, nil
}

// LoadPrivateKeyFromEnv reads the Base64-encoded private key from the
// environment variable named by PrivateKeyEnvVarName and decodes it into an
// ed25519.PrivateKey.
//
// It returns an error if the variable is unset/empty or if the value is not
// valid Base64 (standard encoding). The decoded bytes are interpreted by
// length: a 32-byte value is treated as an Ed25519 seed and expanded via
// ed25519.NewKeyFromSeed, while any other length is used directly as a full
// private key.
func LoadPrivateKeyFromEnv() (ed25519.PrivateKey, error) {
	b64Key := os.Getenv(PrivateKeyEnvVarName)
	if b64Key == "" {
		return nil, fmt.Errorf("environment variable %s is not set", PrivateKeyEnvVarName)
	}
	rawBytes, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		print("Decode Fail")
		return nil, err
	}
	if len(rawBytes) == 32 {
		return ed25519.NewKeyFromSeed(rawBytes), nil
	}
	return ed25519.PrivateKey(rawBytes), nil
}
