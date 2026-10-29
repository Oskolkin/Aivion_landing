package dashboardhome

// stringSliceOrEmpty ensures JSON encodes [] instead of null for missing AI summary arrays.
func stringSliceOrEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
