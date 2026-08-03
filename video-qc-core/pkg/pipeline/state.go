package pipeline

import "video-qc-core/pkg/schema"

type StateTransition struct {
	From schema.Status
	To   schema.Status
}

var DefaultTransitions = []StateTransition{
	{From: schema.StatusQueued, To: schema.StatusProbing},
	{From: schema.StatusProbing, To: schema.StatusDecoding},
	{From: schema.StatusDecoding, To: schema.StatusAnalyzing},
	{From: schema.StatusAnalyzing, To: schema.StatusReporting},
	{From: schema.StatusReporting, To: schema.StatusSuccess},
}

func CanTransition(from, to schema.Status) bool {
	if to == schema.StatusFailed || to == schema.StatusCanceled {
		return true
	}
	for _, transition := range DefaultTransitions {
		if transition.From == from && transition.To == to {
			return true
		}
	}
	return false
}
