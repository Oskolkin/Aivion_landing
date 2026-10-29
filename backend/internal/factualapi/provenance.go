package factualapi

// DeriveSourceContractHash returns the stable contract identity from collection
// run contract fields (key + version + endpoint family). Empty when all inputs
// are empty.
func DeriveSourceContractHash(key, version, endpoint string) string {
	if key == "" && version == "" && endpoint == "" {
		return ""
	}
	return StableHash(key, version, endpoint)
}
