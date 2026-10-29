package dashboardhome

func setBlockApplySource(resp *Response, block, source string) {
	if resp == nil || block == "" || source == "" {
		return
	}
	if resp.blockApplySources == nil {
		resp.blockApplySources = map[string]string{}
	}
	resp.blockApplySources[block] = source
}

func blockApplySource(resp *Response, block string) string {
	if resp == nil || resp.blockApplySources == nil {
		return ""
	}
	return resp.blockApplySources[block]
}
