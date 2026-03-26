package mcpserver

// PrescribeSmartInput is the input schema for the prescribe_smart tool.
type PrescribeSmartInput struct {
	Actor           InputActor        `json:"actor"`
	Tool            string            `json:"tool"`
	Operation       string            `json:"operation"`
	Resource        string            `json:"resource"`
	Namespace       string            `json:"namespace,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	OperationID     string            `json:"operation_id,omitempty"`
	Attempt         int               `json:"attempt,omitempty"`
	TraceID         string            `json:"trace_id,omitempty"`
	SpanID          string            `json:"span_id,omitempty"`
	ParentSpanID    string            `json:"parent_span_id,omitempty"`
	ScopeDimensions map[string]string `json:"scope_dimensions,omitempty"`
}

func (input PrescribeSmartInput) toPrescribeInput() PrescribeInput {
	return PrescribeInput{
		Actor:           input.Actor,
		Tool:            input.Tool,
		Operation:       input.Operation,
		Resource:        input.Resource,
		Namespace:       input.Namespace,
		Environment:     input.Environment,
		SessionID:       input.SessionID,
		OperationID:     input.OperationID,
		Attempt:         input.Attempt,
		TraceID:         input.TraceID,
		SpanID:          input.SpanID,
		ParentSpanID:    input.ParentSpanID,
		ScopeDimensions: input.ScopeDimensions,
	}
}
