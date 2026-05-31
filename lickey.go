package lickey

const PrivateKeyEnvVarName = "lickey_privatekey"

type LicenseData struct {
	Username    string
	SkuID       uint16
	FeatureMask uint8
	ExpiryDate  string // Format: YYYY-MM-DD
}
