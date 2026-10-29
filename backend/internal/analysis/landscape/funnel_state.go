package landscape

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"

// FunnelBuildState collects known/limited/empty funnel metric observations.
type FunnelBuildState struct {
	KnownMetricCount   int
	LimitedMetricCount int
	MissingMetricCount int
	FoundEmptyCount    int
	Limitations        []Limitation
}

func observeFunnelStatus(s *FunnelBuildState, status factualapi.BatchStatus, known bool, code, message string) {
	if s == nil {
		return
	}
	if code != "" {
		s.Limitations = append(s.Limitations, Limitation{Code: code, Message: message})
	}
	switch status {
	case factualapi.BatchLimited:
		s.LimitedMetricCount++
		if known {
			s.KnownMetricCount++
		}
	case factualapi.BatchFoundEmpty:
		s.FoundEmptyCount++
		s.MissingMetricCount++
	case factualapi.BatchFound:
		if known {
			s.KnownMetricCount++
		} else {
			s.MissingMetricCount++
		}
	default:
		s.MissingMetricCount++
	}
}

func observeFunnelValue(s *FunnelBuildState, v factualapi.Value[factualapi.Decimal], status factualapi.BatchStatus) {
	observeFunnelStatus(s, status, v.State == factualapi.ValueKnown && v.Value != nil, "", "")
}

func finalizeFunnelState(s FunnelBuildState) (BlockState, []Limitation) {
	if s.KnownMetricCount > 0 && s.LimitedMetricCount == 0 {
		return BlockReady, s.Limitations
	}
	if s.LimitedMetricCount > 0 {
		return BlockPartial, s.Limitations
	}
	if s.KnownMetricCount == 0 && s.FoundEmptyCount > 0 && s.MissingMetricCount == s.FoundEmptyCount {
		return BlockReadyEmpty, s.Limitations
	}
	if s.KnownMetricCount == 0 && s.MissingMetricCount > 0 {
		return BlockPartial, s.Limitations
	}
	return BlockReadyEmpty, s.Limitations
}
