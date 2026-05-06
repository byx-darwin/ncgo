package protolint

import "fmt"

const requestFieldCountWarningThreshold = 12

type rulePIO101 struct{}

func (rulePIO101) Meta() RuleMeta {
	return RuleMeta{ID: "PIO101", Level: LevelError, Phase: Phase1}
}

func (r rulePIO101) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		expectedIn, expectedOut := expectedReqRespNames(rpc.Name)
		if rpc.InputMessageName == expectedIn && rpc.OutputMessageName == expectedOut {
			continue
		}
		out = append(out, rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("rpc %s must use %s/%s", rpc.Name, expectedIn, expectedOut),
			"rename the RPC input/output messages to match the method-specific Req/Resp convention",
		))
	}
	return out
}

type rulePIO102 struct{}

func (rulePIO102) Meta() RuleMeta {
	return RuleMeta{ID: "PIO102", Level: LevelError, Phase: Phase1}
}

func (r rulePIO102) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		expectedIn, expectedOut := expectedReqRespNames(rpc.Name)
		if rpc.InputMessageName != expectedIn {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s input %s does not match expected %s", rpc.Name, rpc.InputMessageName, expectedIn),
				"rename the RPC input message so it matches the method name",
			)
			d.Message = rpc.InputMessageName
			out = append(out, d)
		}
		if rpc.OutputMessageName != expectedOut {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s output %s does not match expected %s", rpc.Name, rpc.OutputMessageName, expectedOut),
				"rename the RPC output message so it matches the method name",
			)
			d.Message = rpc.OutputMessageName
			out = append(out, d)
		}
	}
	return out
}

func expectedReqRespNames(method string) (string, string) {
	return method + "Req", method + "Resp"
}

type rulePIO111 struct{}

func (rulePIO111) Meta() RuleMeta {
	return RuleMeta{ID: "PIO111", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO111) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if isGoogleProtobufEmpty(rpc.InputMessage, rpc.InputMessageName) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s uses google.protobuf.Empty as input", rpc.Name),
				"prefer an explicit empty <Method>Req message when the RPC is part of your public business contract",
			)
			d.Message = "google.protobuf.Empty"
			out = append(out, d)
		}
		if isGoogleProtobufEmpty(rpc.OutputMessage, rpc.OutputMessageName) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s uses google.protobuf.Empty as output", rpc.Name),
				"prefer an explicit empty <Method>Resp message when the RPC is part of your public business contract",
			)
			d.Message = "google.protobuf.Empty"
			out = append(out, d)
		}
	}
	return out
}

type rulePIO112 struct{}

func (rulePIO112) Meta() RuleMeta {
	return RuleMeta{ID: "PIO112", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO112) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if isGenericTopLevelMessageName(rpc.InputMessageName) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s input %s looks too generic for a top-level request", rpc.Name, rpc.InputMessageName),
				"use a method-specific request message instead of a reusable generic top-level input",
			)
			d.Message = rpc.InputMessageName
			out = append(out, d)
		}
		if isGenericTopLevelMessageName(rpc.OutputMessageName) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s output %s looks too generic for a top-level response", rpc.Name, rpc.OutputMessageName),
				"use a method-specific response message instead of a reusable generic top-level output",
			)
			d.Message = rpc.OutputMessageName
			out = append(out, d)
		}
	}
	return out
}

type rulePIO113 struct{}

func (rulePIO113) Meta() RuleMeta {
	return RuleMeta{ID: "PIO113", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO113) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.InputMessage == nil {
			continue
		}
		fieldCount := len(rpc.InputMessage.Fields)
		if fieldCount <= requestFieldCountWarningThreshold {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("request %s declares %d fields, which exceeds the warning threshold %d", rpc.InputMessage.Name, fieldCount, requestFieldCountWarningThreshold),
			"consider splitting the request or grouping related inputs so the RPC contract stays focused",
		)
		d.Message = rpc.InputMessage.Name
		out = append(out, d)
	}
	return out
}

func isGenericTopLevelMessageName(name string) bool {
	switch name {
	case "CommonReq", "CommonResp", "BaseResp", "Result":
		return true
	default:
		return false
	}
}

func isGoogleProtobufEmpty(msg *Message, name string) bool {
	if msg != nil && msg.FullName == "google.protobuf.Empty" {
		return true
	}
	return name == "Empty" || name == "google.protobuf.Empty"
}
