package protolint

// Level classifies the consequence of a lint finding.
type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

// Phase groups rules by planned rollout stage.
type Phase string

const (
	Phase1 Phase = "phase1"
	Phase2 Phase = "phase2"
	Phase3 Phase = "phase3"
)

// Diagnostic is one structured lint finding.
type Diagnostic struct {
	RuleID  string `json:"ruleId"`
	Level   Level  `json:"level"`
	Phase   Phase  `json:"phase"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Service string `json:"service,omitempty"`
	RPC     string `json:"rpc,omitempty"`
	Message string `json:"message,omitempty"`
	Field   string `json:"field,omitempty"`
	Summary string `json:"summary"`
	Hint    string `json:"hint,omitempty"`
}

// RuleMeta is the stable metadata associated with one lint rule.
type RuleMeta struct {
	ID    string
	Level Level
	Phase Phase
}

func rpcDiagnostic(meta RuleMeta, rpc *RPC, summary, hint string) Diagnostic {
	d := Diagnostic{
		RuleID:  meta.ID,
		Level:   meta.Level,
		Phase:   meta.Phase,
		Summary: summary,
		Hint:    hint,
	}
	if rpc == nil {
		return d
	}
	d.File = fileForRPC(rpc)
	d.Line = rpc.Location.Line
	d.Column = rpc.Location.Column
	d.Service = rpc.ServiceName
	d.RPC = rpc.Name
	return d
}

func fileForRPC(rpc *RPC) string {
	if rpc == nil {
		return ""
	}
	if rpc.File != "" {
		return rpc.File
	}
	if rpc.InputMessage != nil && rpc.InputMessage.File != "" {
		return rpc.InputMessage.File
	}
	if rpc.OutputMessage != nil && rpc.OutputMessage.File != "" {
		return rpc.OutputMessage.File
	}
	return ""
}
