package eval

func ArtifactKey(kind, name string) string {
	switch kind {
	case KindFactualGolden:
		return ArtifactNS + "factual-golden/" + name
	case KindCapabilityMatrix:
		return ArtifactNS + "capability/" + name
	case KindAIGolden:
		return ArtifactNS + "ai-golden/" + name
	case KindLongitudinal:
		return ArtifactNS + "longitudinal/" + name
	case KindChatGolden:
		return ArtifactNS + "chat/" + name
	case KindAttachmentGolden:
		return ArtifactNS + "attachments/" + name
	case KindRealSellerReplay:
		return ArtifactNS + "real-replay/" + name
	case KindGraderCalibration:
		return ArtifactNS + "calibration/" + name
	default:
		return ArtifactNS + "runs/" + name
	}
}

func RegressionSuites(change string) []string {
	switch change {
	case "sales_formula":
		return []string{KindFactualGolden, KindChatGolden}
	case "materiality":
		return []string{KindAIGolden, KindLongitudinal}
	case "discovery_prompt", "discovery_model":
		return []string{KindAIGolden}
	case "evidence":
		return []string{KindAIGolden}
	case "deep_prompt", "deep_model":
		return []string{KindAIGolden, KindLongitudinal}
	case "action_catalog":
		return []string{KindLongitudinal}
	case "chat_prompt", "chat_tool", "chat_model":
		return []string{KindChatGolden, KindAttachmentGolden}
	case "funnel":
		return []string{KindCapabilityMatrix, KindFactualGolden}
	case "model_change":
		return []string{KindFactualGolden, KindCapabilityMatrix, KindAIGolden, KindLongitudinal, KindChatGolden, KindAttachmentGolden}
	default:
		return []string{KindFactualGolden}
	}
}
