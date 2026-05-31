package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
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

	fmt.Println(privKey) //REMOVE BEFORE RELEASE
	fmt.Println(lic)     //REMOVE BEFORE RELEASE
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
	println(b64Key) //REMOVE BEFORE RELEASE
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
