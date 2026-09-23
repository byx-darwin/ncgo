package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	ncgoexec "github.com/byx-darwin/ncgo/internal/exec"
)

const (
	mcpOutputText  = "text"
	mcpOutputJSON  = "json"
	mcpOutputSARIF = "sarif"

	mcpResultSchemaVersion = "ncgo.mcp.result/v1"
	mcpInternalErrorCode   = "mcp_internal_error"
	mcpInvalidArgsCode     = "mcp_invalid_arguments"
	mcpValidationErrorCode = "mcp_validation_failed"
	mcpDependencyErrorCode = "mcp_dependency_missing"
	mcpNetworkErrorCode    = "mcp_network_error"
	mcpExternalErrorCode   = "mcp_external_process_failed"
	mcpNotFoundErrorCode   = "mcp_resource_not_found"
)

// mcpError is the stable, machine-readable error contract returned by every
// tool failure. Path is optional because not every failure can be attributed
// to one filesystem entry.
type mcpError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Retryable   bool   `json:"retryable"`
	Path        string `json:"path,omitempty"`
	Remediation string `json:"remediation"`
}

type outputWriter func(io.Writer) error

type structuredMCPTool[T any] struct {
	name      string
	supported []string
	format    func(T, string) (string, error)
	fields    func(T) map[string]any
	isError   func(T) bool
}

func (t structuredMCPTool[T]) resolveOutput(output string) (string, error) {
	return resolveMCPOutput(t.name, output, t.supported...)
}

func (t structuredMCPTool[T]) buildResult(res T, output string) (map[string]any, error) {
	text, err := t.format(res, output)
	if err != nil {
		return nil, err
	}
	isError := t.isError(res)
	result := buildMCPResult(text, isError, t.fields(res))
	if isError {
		setResultError(result, mcpError{
			Code:        toolFailureCode(t.name),
			Message:     text,
			Retryable:   false,
			Remediation: "Inspect the structured diagnostics, fix the reported " + t.name + " failures, and retry.",
		})
	}
	return result, nil
}

func resolveMCPOutput(toolName, output string, supported ...string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return mcpOutputText, nil
	}
	if slices.Contains(supported, output) {
		return output, nil
	}
	return "", fmt.Errorf("%s: unsupported output %q; want %s", toolName, output, formatOutputList(supported))
}

func formatMCPOutput(output string, writers map[string]outputWriter) (string, error) {
	write, ok := writers[output]
	if !ok {
		return "", fmt.Errorf("mcp: missing renderer for output %q", output)
	}
	var buf strings.Builder
	if err := write(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeJSONOutput(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func buildMCPResult(text string, isError bool, fields map[string]any) map[string]any {
	out := textResult(text, isError)
	// Keep the pre-v1 sibling fields during the migration period. New clients
	// should consume structuredContent; older clients can continue reading the
	// same fields at the result top level. Reserved protocol fields cannot be
	// replaced by a tool-specific payload.
	for key, value := range fields {
		if !reservedCallToolResultField(key) {
			out[key] = value
		}
	}
	out["structuredContent"] = buildMCPEnvelope(isError, fields, envelopeError(out))
	return out
}

func textResult(text string, isError bool) map[string]any {
	var toolErr any
	if isError {
		toolErr = mcpError{
			Code:        mcpInternalErrorCode,
			Message:     text,
			Retryable:   false,
			Remediation: "Review the error message and tool arguments, then retry.",
		}
	}
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": text}},
		"structuredContent": buildMCPEnvelope(isError, nil, toolErr),
		"isError":           isError,
	}
}

func toolErrorResult(code, message string, retryable bool, path, remediation string) map[string]any {
	result := textResult(message, true)
	toolErr := mcpError{Code: code, Message: message, Retryable: retryable, Path: path, Remediation: remediation}
	setResultError(result, toolErr)
	return result
}

func invalidArgumentResult(message string) map[string]any {
	return toolErrorResult(mcpInvalidArgsCode, message, false, "", "Correct the tool arguments using its inputSchema, then retry.")
}

func validationErrorResult(operation string, err error) map[string]any {
	return toolErrorResult(mcpValidationErrorCode, err.Error(), false, "", "Fix the project state or conflicting input reported by "+operation+", then retry.")
}

func resourceNotFoundResult(message string) map[string]any {
	return toolErrorResult(mcpNotFoundErrorCode, message, false, "", "Choose an existing resource, list available resources if supported, then retry.")
}

func operationErrorResult(operation string, err error) map[string]any {
	return toolErrorResult(toolFailureCode(operation), err.Error(), false, "", "Resolve the reported "+operation+" failure, then retry.")
}

func dependencyErrorResult(message string) map[string]any {
	result := toolErrorResult(mcpDependencyErrorCode, message, false, "", "Install the missing dependency, verify PATH, then retry.")
	setResultEffects(result, []mcpEffect{{Kind: "dependency", Action: "resolve", Status: "failed", Detail: message}})
	return result
}

func networkErrorResult(operation string, err error) map[string]any {
	result := toolErrorResult(mcpNetworkErrorCode, err.Error(), true, "", "Check network access and the remote registry, then retry "+operation+".")
	setResultEffects(result, []mcpEffect{{Kind: "network", Action: operation, Status: "failed", Detail: err.Error()}})
	return result
}

func externalProcessErrorResult(operation string, err error) map[string]any {
	result := toolErrorResult(mcpExternalErrorCode, err.Error(), false, "", "Inspect the external command output, fix its inputs or environment, then retry "+operation+".")
	setResultEffects(result, []mcpEffect{{Kind: "process", Action: operation, Status: "failed", Detail: err.Error()}})
	return result
}

func classifiedOperationErrorResult(operation string, err error) map[string]any {
	var notFound *ncgoexec.NotFoundError
	if errors.As(err, &notFound) {
		return dependencyErrorResult(err.Error())
	}
	var exitErr *ncgoexec.ExitError
	if errors.As(err, &exitErr) {
		return externalProcessErrorResult(operation, err)
	}
	return operationErrorResult(operation, err)
}

func setResultError(result map[string]any, toolErr mcpError) {
	result["error"] = toolErr
	if envelope, ok := result["structuredContent"].(map[string]any); ok {
		envelope["error"] = toolErr
		envelope["ok"] = false
	}
	result["isError"] = true
}

func toolFailureCode(name string) string {
	normalized := strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(strings.ToLower(strings.TrimSpace(name)))
	if normalized == "" {
		return mcpInternalErrorCode
	}
	return "ncgo_" + normalized + "_failed"
}

func resultSlice(fields map[string]any, key string) any {
	if value, ok := fields[key]; ok && value != nil {
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return []any{}
		}
		return value
	}
	return []any{}
}

func buildMCPEnvelope(isError bool, fields map[string]any, toolErr any) map[string]any {
	data := make(map[string]any, len(fields))
	for key, value := range fields {
		if !reservedEnvelopeField(key) {
			data[key] = value
		}
	}
	return map[string]any{
		"schemaVersion": mcpResultSchemaVersion,
		"ok":            !isError,
		"data":          data,
		"effects":       resultSlice(fields, "effects"),
		"diagnostics":   resultSlice(fields, "diagnostics"),
		"error":         toolErr,
		"nextSteps":     resultSlice(fields, "nextSteps"),
	}
}

func envelopeError(result map[string]any) any {
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		return nil
	}
	return structured["error"]
}

func reservedCallToolResultField(key string) bool {
	switch key {
	case "content", "structuredContent", "isError", "_meta":
		return true
	default:
		return false
	}
}

func reservedEnvelopeField(key string) bool {
	switch key {
	case "schemaVersion", "ok", "data", "effects", "diagnostics", "error", "nextSteps", "content", "structuredContent", "isError", "_meta":
		return true
	default:
		return false
	}
}

func formatOutputList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}
