#include "LicKeyVerifier.h"

#include "Misc/Base64.h"

// OpenSSL ships with the engine; wrap its headers so UE's strict warning and
// platform macros do not fight with the third-party code.
THIRD_PARTY_INCLUDES_START
#include <openssl/evp.h>
#include <openssl/sha.h>
THIRD_PARTY_INCLUDES_END

namespace
{
	// Layout of the decoded bundle, mirroring the LicKey generator.
	constexpr int32 PayloadLen   = 11; // 4-byte user hash + 2-byte SKU + 1 feature + 4-byte expiry
	constexpr int32 SignatureLen = 64; // Ed25519 signature
	constexpr int32 BundleLen    = PayloadLen + SignatureLen; // 75 bytes
	constexpr int32 Ed25519PubLen = 32;

	/** RFC 4648 Base32 (standard alphabet, no padding) decoder. */
	bool Base32Decode(const FString& In, TArray<uint8>& Out)
	{
		auto CharVal = [](TCHAR C) -> int32
		{
			if (C >= 'A' && C <= 'Z') { return C - 'A'; }
			if (C >= 'a' && C <= 'z') { return C - 'a'; }      // tolerate lower case
			if (C >= '2' && C <= '7') { return (C - '2') + 26; }
			return -1;
		};

		uint32 Buffer = 0;
		int32 BitsLeft = 0;
		for (const TCHAR C : In)
		{
			// Skip the cosmetic hyphens and any whitespace.
			if (C == '-' || C == ' ' || C == '\t' || C == '\r' || C == '\n')
			{
				continue;
			}

			const int32 Val = CharVal(C);
			if (Val < 0)
			{
				return false; // illegal character
			}

			Buffer = (Buffer << 5) | static_cast<uint32>(Val);
			BitsLeft += 5;
			if (BitsLeft >= 8)
			{
				BitsLeft -= 8;
				Out.Add(static_cast<uint8>((Buffer >> BitsLeft) & 0xFF));
			}
		}
		return true;
	}

	/** First 4 bytes of SHA-256 over the trimmed, lower-cased username. */
	void HashUsername(const FString& Username, uint8 OutHash[4])
	{
		const FString Normalised = Username.TrimStartAndEnd().ToLower();
		const FTCHARToUTF8 Utf8(*Normalised);

		uint8 Digest[SHA256_DIGEST_LENGTH];
		SHA256(reinterpret_cast<const unsigned char*>(Utf8.Get()), Utf8.Length(), Digest);
		FMemory::Memcpy(OutHash, Digest, 4);
	}

	/** Verify an Ed25519 signature using engine OpenSSL. */
	bool Ed25519Verify(const uint8* PubKey, const uint8* Message, int32 MessageLen, const uint8* Signature)
	{
		EVP_PKEY* PKey = EVP_PKEY_new_raw_public_key(EVP_PKEY_ED25519, nullptr, PubKey, Ed25519PubLen);
		if (PKey == nullptr)
		{
			return false;
		}

		bool bOk = false;
		EVP_MD_CTX* Ctx = EVP_MD_CTX_new();
		if (Ctx != nullptr)
		{
			if (EVP_DigestVerifyInit(Ctx, nullptr, nullptr, nullptr, PKey) == 1)
			{
				bOk = EVP_DigestVerify(Ctx, Signature, SignatureLen, Message, static_cast<size_t>(MessageLen)) == 1;
			}
			EVP_MD_CTX_free(Ctx);
		}

		EVP_PKEY_free(PKey);
		return bOk;
	}
}

ELicKeyStatus ULicKeyVerifier::VerifyLicenseKey(const FString& LicenseKey, const FString& Base64PublicKey, FLicKeyDetails& OutDetails)
{
	OutDetails = FLicKeyDetails();

	// 1. Decode the issuer public key.
	TArray<uint8> PubKey;
	if (!FBase64::Decode(Base64PublicKey.TrimStartAndEnd(), PubKey) || PubKey.Num() != Ed25519PubLen)
	{
		return ELicKeyStatus::InvalidPublicKey;
	}

	// 2. Base32-decode the key text into the raw 75-byte bundle.
	TArray<uint8> Bundle;
	if (!Base32Decode(LicenseKey, Bundle) || Bundle.Num() != BundleLen)
	{
		return ELicKeyStatus::MalformedKey;
	}

	const uint8* Payload = Bundle.GetData();
	const uint8* Signature = Bundle.GetData() + PayloadLen;

	// 3. The signature is the tamper check: any change to the payload fields
	//    invalidates it against the issuer's key.
	if (!Ed25519Verify(PubKey.GetData(), Payload, PayloadLen, Signature))
	{
		return ELicKeyStatus::SignatureInvalid;
	}

	// 4. Unpack the little-endian payload.
	OutDetails.UserHash.Append(Payload, 4);
	OutDetails.UserHashHex = FString::Printf(TEXT("%02x%02x%02x%02x"), Payload[0], Payload[1], Payload[2], Payload[3]);
	OutDetails.SkuID = static_cast<int32>(Payload[4]) | (static_cast<int32>(Payload[5]) << 8);
	OutDetails.FeatureMask = static_cast<int32>(Payload[6]);

	const uint32 ExpiryUnix =
		  static_cast<uint32>(Payload[7])
		| (static_cast<uint32>(Payload[8]) << 8)
		| (static_cast<uint32>(Payload[9]) << 16)
		| (static_cast<uint32>(Payload[10]) << 24);

	OutDetails.Expiry = FDateTime::FromUnixTimestamp(static_cast<int64>(ExpiryUnix));
	OutDetails.DaysRemaining = static_cast<int32>((OutDetails.Expiry - FDateTime::UtcNow()).GetTotalDays());

	// 5. Report expiry status.
	if (FDateTime::UtcNow() > OutDetails.Expiry)
	{
		return ELicKeyStatus::Expired;
	}

	return ELicKeyStatus::Valid;
}

bool ULicKeyVerifier::VerifyUsername(const FString& Username, const FLicKeyDetails& Details)
{
	if (Details.UserHash.Num() != 4)
	{
		return false;
	}

	uint8 Candidate[4];
	HashUsername(Username, Candidate);
	return FMemory::Memcmp(Candidate, Details.UserHash.GetData(), 4) == 0;
}

bool ULicKeyVerifier::HasFeature(const FLicKeyDetails& Details, int32 FeatureBit)
{
	if (FeatureBit < 0 || FeatureBit > 7)
	{
		return false;
	}
	return (Details.FeatureMask & (1 << FeatureBit)) != 0;
}
