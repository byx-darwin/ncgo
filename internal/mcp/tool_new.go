package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/postgenerate"
	"github.com/byx-darwin/ncgo/internal/registry"
	"github.com/byx-darwin/ncgo/internal/scaffold/micro"
	"github.com/byx-darwin/ncgo/internal/scaffold/mono"
)

var newMCPTool = structuredMCPTool[*newResult]{
	name:      "new",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPNewOutput,
	fields: func(res *newResult) map[string]any {
		out := map[string]any{
			"dir":       res.Dir,
			"nextSteps": res.NextSteps,
			"mode":      res.Mode,
		}
		if res.RanGenerate != nil {
			out["ranGenerate"] = *res.RanGenerate
		}
		return out
	},
	isError: func(*newResult) bool { return false },
}

// newResult wraps mono.Result and micro.Result into a single MCP output shape.
type newResult struct {
	Dir         string
	NextSteps   []string
	Mode        string
	RanGenerate *bool                     // only set for mono mode
	AutoSteps   []postgenerate.StepResult `json:",omitempty"`
}

func callNew(ctx context.Context, raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Name           string   `json:"name"`
		Module         string   `json:"module"`
		Dir            string   `json:"dir"`
		Mode           string   `json:"mode"`
		Kind           string   `json:"kind"`
		DB             string   `json:"db"`
		Infra          []string `json:"infra"`
		NoGenerate     bool     `json:"noGenerate"`
		Preset         string   `json:"preset"`
		RuleCenterAddr string   `json:"ruleCenterAddr"`
		Template       string   `json:"template"`
		TemplateDir    string   `json:"templateDir"`
		Output         string   `json:"output"`
		AITarget       string   `json:"aiTarget"`
		NoAutoSteps    bool     `json:"noAutoSteps"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Module == "" {
		return textResult("module is required (--module)", true), nil
	}
	if args.Name == "" {
		return textResult("name is required", true), nil
	}
	if args.Mode == "" {
		args.Mode = manifest.ModeMono
	}

	output, err := newMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	var res *newResult
	dir := args.Name
	if args.Dir != "" {
		dir = args.Dir
	}
	switch args.Mode {
	case manifest.ModeMono:
		res, err = runNewMono(ctx, args.Name, args.Module, dir, args.Kind, args.DB, args.Infra, args.NoGenerate, args.Preset, args.RuleCenterAddr, args.AITarget, args.NoAutoSteps, ncgoVersion, assetsVersion)
	case manifest.ModeMicro:
		var templateDir string
		templateDir, err = registry.ResolveTemplateDir(args.Template, args.TemplateDir)
		if err != nil {
			return textResult(err.Error(), true), nil
		}
		res, err = runNewMicro(args.Name, args.Module, dir, ncgoVersion, assetsVersion, templateDir)
	default:
		return textResult(fmt.Sprintf("mode %q is invalid (mono|micro)", args.Mode), true), nil
	}
	if err != nil {
		var nf *exec.NotFoundError
		if errors.As(err, &nf) {
			return textResult(fmt.Sprintf("generator tool %q not found on PATH. Install: %s", nf.Name, exec.InstallHint(nf.Name)), true), nil
		}
		return textResult(err.Error(), true), nil
	}
	out, err := newMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func runNewMono(ctx context.Context, name, module, dir, kind, db string, infra []string, noGenerate bool, preset, ruleCenterAddr, aiTarget string, noAutoSteps bool, ncgoVersion, assetsVersion string) (*newResult, error) {
	if kind == "" {
		kind = manifest.KindHertz
	}
	if db == "" {
		db = "none"
	}
	res, err := mono.Generate(ctx, mono.Options{
		Name:           name,
		Module:         module,
		Kind:           kind,
		Dir:            dir,
		WithDatabase:   db == "postgres",
		Infra:          infra,
		Preset:         preset,
		RuleCenterAddr: ruleCenterAddr,
		AssetsVersion:  assetsVersion,
		NCGOVersion:    ncgoVersion,
		NoGenerate:     noGenerate,
	})
	if err != nil {
		return nil, err
	}

	// Run auto post-generation steps
	var autoSteps []postgenerate.StepResult
	if res.RanGenerate {
		pgResult := postgenerate.Run(postgenerate.Options{
			Dir:         res.Dir,
			AITarget:    aiTarget,
			NoAutoSteps: noAutoSteps,
			RanGenerate: res.RanGenerate,
			Stdout:      io.Discard, // MCP doesn't print progress
		})
		autoSteps = pgResult.Steps
	}

	ran := res.RanGenerate
	return &newResult{
		Dir:         res.Dir,
		NextSteps:   res.NextSteps,
		Mode:        manifest.ModeMono,
		RanGenerate: &ran,
		AutoSteps:   autoSteps,
	}, nil
}

func runNewMicro(name, module, dir, ncgoVersion, assetsVersion, templateDir string) (*newResult, error) {
	res, err := micro.Generate(micro.Options{
		Name:          name,
		Module:        module,
		Dir:           dir,
		AssetsVersion: assetsVersion,
		NCGOVersion:   ncgoVersion,
		TemplateDir:   templateDir,
	})
	if err != nil {
		return nil, err
	}
	return &newResult{
		Dir:       res.Dir,
		NextSteps: res.NextSteps,
		Mode:      manifest.ModeMicro,
	}, nil
}

func formatMCPNewOutput(res *newResult, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "scaffolded %s %s at %s\n\nnext steps:\n", res.Mode, res.Mode, res.Dir)
			if err != nil {
				return err
			}
			for _, s := range res.NextSteps {
				if _, err := fmt.Fprintf(w, "  $ %s\n", s); err != nil {
					return err
				}
			}
			return nil
		},
		mcpOutputJSON: func(w io.Writer) error {
			return writeJSONOutput(w, map[string]any{
				"dir":       res.Dir,
				"nextSteps": res.NextSteps,
				"mode":      res.Mode,
				"ranGenerate": func() any {
					if res.RanGenerate != nil {
						return *res.RanGenerate
					}
					return nil
				}(),
			})
		},
	})
}
