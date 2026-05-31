# LicKey Client (Unreal Engine)

A drop-in C++ Blueprint Function Library that consumes a license key produced by
the [LicKey generator](../../lickey.go), verifies its Ed25519 signature against
your issuer public key, and exposes the embedded license fields to both C++ and
Blueprints inside your game.

It uses the **OpenSSL** module that already ships with Unreal Engine, so there
are no extra third-party libraries to vendor in.

> Tested against Unreal Engine 5.x. The crypto APIs used
> (`EVP_PKEY_new_raw_public_key`, `EVP_DigestVerify`, `SHA256`) are available in
> the OpenSSL version bundled with UE 5.0 and later.

## Files

| File                                                   | Purpose                                              |
| ------------------------------------------------------ | ---------------------------------------------------- |
| [`Source/LicKeyVerifier.h`](Source/LicKeyVerifier.h)   | `ULicKeyVerifier` library, `FLicKeyDetails`, status enum. |
| [`Source/LicKeyVerifier.cpp`](Source/LicKeyVerifier.cpp) | Base32 decode, Ed25519 verify, payload parsing.    |

## Installing into your game

1. **Copy the source.** Drop `LicKeyVerifier.h` and `LicKeyVerifier.cpp` into
   your game module's source folder, e.g.
   `YourGame/Source/YourGame/Private/Licensing/` (header into `Public/` if you
   prefer the conventional split).

2. **Depend on OpenSSL.** In your module's `*.Build.cs`, add `OpenSSL` to the
   private dependencies:

   ```csharp
   PrivateDependencyModuleNames.AddRange(new string[]
   {
       "OpenSSL",
   });
   ```

3. **Embed your public key.** The generator prints a Base64 Ed25519 public key
   when you first run it. Paste *only the public key* into your game (e.g. a
   config value or a constant). Never ship the private key.

4. **Regenerate project files** (right-click the `.uproject` → *Generate Visual
   Studio project files*) and rebuild.

## Usage (C++)

```cpp
#include "LicKeyVerifier.h"

const FString PublicKey = TEXT("LbpQTLoslOmIE8d0miQRBJi97bnpLO7Uu3FGpdMuKrA=");

FLicKeyDetails Details;
const ELicKeyStatus Status = ULicKeyVerifier::VerifyLicenseKey(UserEnteredKey, PublicKey, Details);

switch (Status)
{
case ELicKeyStatus::Valid:
    UE_LOG(LogTemp, Display, TEXT("Licensed: SKU %d, %d day(s) left"), Details.SkuID, Details.DaysRemaining);

    // Gate features off the bitmask (bit 0 = Feature A, bit 1 = Feature B, ...).
    if (ULicKeyVerifier::HasFeature(Details, /*FeatureBit=*/0))
    {
        EnableFeatureA();
    }

    // Optionally bind the key to a known account name.
    if (ULicKeyVerifier::VerifyUsername(TEXT("player@example.com"), Details))
    {
        UE_LOG(LogTemp, Display, TEXT("Username matches embedded hash."));
    }
    break;

case ELicKeyStatus::Expired:
    UE_LOG(LogTemp, Warning, TEXT("License expired on %s"), *Details.Expiry.ToString());
    break;

default:
    UE_LOG(LogTemp, Error, TEXT("Activation failed: %d"), static_cast<int32>(Status));
    break;
}
```

## Usage (Blueprint)

All three functions are exposed under the **LicKey** category:

- **Verify License Key** — input the user's key string and your Base64 public
  key; returns an `ELicKeyStatus` and a `FLicKeyDetails` struct (`SkuID`,
  `FeatureMask`, `Expiry`, `DaysRemaining`, `UserHashHex`).
- **Has Feature** — pure node; pass the details struct and a feature bit (0-7).
- **Verify Username** — pass a candidate username and the details struct.

A typical activation graph: `Verify License Key` → `Switch on ELicKeyStatus` →
on `Valid`, store the unlocked features; on anything else, show an error and keep
the game locked.

## API

### `ELicKeyStatus VerifyLicenseKey(LicenseKey, Base64PublicKey, out Details)`

| Status             | Meaning                                                       |
| ------------------ | ------------------------------------------------------------- |
| `Valid`            | Signature valid and not expired. `Details` populated.         |
| `Expired`          | Signature valid but past the expiry date. `Details` populated.|
| `SignatureInvalid` | Decoded but the signature does not match your public key.     |
| `MalformedKey`     | Key text is not valid Base32 / not 75 bytes.                  |
| `InvalidPublicKey` | Public key was not valid Base64 or not 32 bytes.              |

The license key input tolerates the cosmetic hyphens, surrounding whitespace,
and lower-case letters, so you can feed it straight from a text box.

### `bool VerifyUsername(Username, Details)`

The key stores only a 4-byte SHA-256 hash of the username (matching the
generator's *trimmed, lower-cased* normalisation), so the name cannot be
recovered from the key. This re-hashes a candidate name and compares it against
the embedded hash.

### `bool HasFeature(Details, FeatureBit)`

Returns whether the zero-based feature bit (0-7) is set in the feature mask.

## How verification works

The decoder mirrors the generator's encoding exactly:

1. Strip hyphens/whitespace and **Base32-decode** (standard alphabet, no
   padding) the text into a raw **75-byte bundle**.
2. Split into an **11-byte payload** and a **64-byte Ed25519 signature**.
3. **Verify** the payload against your public key — this is the tamper check.
   Any change to the SKU, feature mask, expiry, or user hash breaks it.
4. Unpack the little-endian payload:

   | Byte range | Field          | Type        |
   | ---------- | -------------- | ----------- |
   | 0 – 3      | Username hash  | `uint8[4]`  |
   | 4 – 5      | SKU ID         | `uint16` LE |
   | 6          | Feature mask   | `uint8`     |
   | 7 – 10     | Expiry (Unix)  | `uint32` LE |
   | 11 – 74    | Signature      | `uint8[64]` |

5. Compare the embedded expiry against the current UTC time.

See the [project README](../../README.md#structure-of-a-license-key) for the
full byte-layout rationale.

## Security notes

- **Ship only the public key.** The private key signs licenses and must stay on
  your build/signing machine (the `lickey_privatekey` environment variable).
- **Client-side verification is advisory.** A determined attacker controls the
  binary and can patch out any check. Ed25519 verification proves a key was
  signed by *you* and is great for honest-customer activation and feature
  gating, but it is not a substitute for server-side entitlement checks if you
  need hard anti-piracy guarantees.
- Verification is local and offline — no network calls are made.
