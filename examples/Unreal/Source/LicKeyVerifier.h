// LicKey verifier for Unreal Engine.
//
// Drop-in Blueprint Function Library that consumes a license key produced by
// the LicKey generator (../../lickey.go), verifies its Ed25519 signature
// against an issuer public key, and exposes the embedded license fields to
// both C++ and Blueprints.
//
// Crypto is provided by the OpenSSL module that ships with Unreal Engine, so
// there are no extra third-party dependencies. Remember to add "OpenSSL" to
// your module's PrivateDependencyModuleNames (see README.md).

#pragma once

#include "CoreMinimal.h"
#include "Kismet/BlueprintFunctionLibrary.h"
#include "LicKeyVerifier.generated.h"

/** Result of attempting to verify a license key. */
UENUM(BlueprintType)
enum class ELicKeyStatus : uint8
{
	/** Signature valid and the license has not expired. */
	Valid			UMETA(DisplayName = "Valid"),
	/** Key text could not be Base32-decoded or is the wrong length. */
	MalformedKey	UMETA(DisplayName = "Malformed Key"),
	/** Supplied public key was not valid Base64 or the wrong size. */
	InvalidPublicKey UMETA(DisplayName = "Invalid Public Key"),
	/** Decoded fine, but the signature does not match the public key. */
	SignatureInvalid UMETA(DisplayName = "Signature Invalid"),
	/** Signature valid, but the embedded expiry date is in the past. */
	Expired			UMETA(DisplayName = "Expired"),
};

/**
 * Fields recovered from a decoded license key.
 *
 * The username itself is never stored in the key, only a 4-byte SHA-256 hash
 * of it (UserHashHex). Use ULicKeyVerifier::VerifyUsername to test a candidate
 * name against that hash.
 */
USTRUCT(BlueprintType)
struct FLicKeyDetails
{
	GENERATED_BODY()

	/** Lower-case hex of the 4-byte username hash embedded in the key. */
	UPROPERTY(BlueprintReadOnly, Category = "LicKey")
	FString UserHashHex;

	/** Product SKU identifier (0-65535). */
	UPROPERTY(BlueprintReadOnly, Category = "LicKey")
	int32 SkuID = 0;

	/** Feature bitmask (0-255); each bit toggles one feature. */
	UPROPERTY(BlueprintReadOnly, Category = "LicKey")
	int32 FeatureMask = 0;

	/** Expiry date (UTC), decoded from the embedded Unix timestamp. */
	UPROPERTY(BlueprintReadOnly, Category = "LicKey")
	FDateTime Expiry;

	/** Whole days remaining until expiry; negative if already expired. */
	UPROPERTY(BlueprintReadOnly, Category = "LicKey")
	int32 DaysRemaining = 0;

	/** Raw 4-byte username hash, kept for username comparison. */
	TArray<uint8> UserHash;
};

/**
 * Blueprint-callable helpers for verifying LicKey license keys at runtime.
 */
UCLASS()
class ULicKeyVerifier : public UBlueprintFunctionLibrary
{
	GENERATED_BODY()

public:
	/**
	 * Verify a hyphen-chunked license key against a Base64 Ed25519 public key.
	 *
	 * @param LicenseKey		The key string as typed by the user (hyphens,
	 *							case and surrounding whitespace are tolerated).
	 * @param Base64PublicKey	The issuer's Base64-encoded Ed25519 public key.
	 * @param OutDetails		Populated with the decoded fields when the
	 *							signature is valid (Valid or Expired).
	 * @return The verification status. Treat anything other than Valid as a
	 *			failed activation.
	 */
	UFUNCTION(BlueprintCallable, Category = "LicKey")
	static ELicKeyStatus VerifyLicenseKey(const FString& LicenseKey, const FString& Base64PublicKey, FLicKeyDetails& OutDetails);

	/**
	 * Re-hash a candidate username and compare it against the 4-byte hash
	 * embedded in an already-decoded key.
	 */
	UFUNCTION(BlueprintCallable, Category = "LicKey")
	static bool VerifyUsername(const FString& Username, const FLicKeyDetails& Details);

	/** Returns true if the given zero-based feature bit is set in the mask. */
	UFUNCTION(BlueprintPure, Category = "LicKey")
	static bool HasFeature(const FLicKeyDetails& Details, int32 FeatureBit);
};
