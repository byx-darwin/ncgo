package infra

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// k8sTemplateData holds the values passed to k8s YAML templates.
type k8sTemplateData struct {
	ServiceName string
	Port        string
	Module      string
}

const (
	k8sSourceDir  = "optional/k8s"
	k8sOutputDir  = "deploy/k8s"
	k8sTmplSuffix = ".tmpl"
)

// addK8s generates the deploy/k8s/ directory tree with Kustomize overlay files.
func addK8s(opts Options, m *manifest.Manifest) (*Result, error) {
	port := "8080"
	if m.Service.Kind == manifest.KindKitex {
		port = "8888"
	}

	data := k8sTemplateData{
		ServiceName: m.Service.Name,
		Port:        port,
		Module:      m.Module,
	}

	type plannedFile struct {
		srcPath string // embedded source path (e.g., optional/k8s/base/deployment.yaml.tmpl)
		dstPath string // absolute output path
		action  string // create or overwrite
	}

	var planned []plannedFile
	err := fs.WalkDir(assets.FS(), k8sSourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, k8sTmplSuffix) {
			return nil
		}
		// Determine output path: strip .tmpl suffix and replace source dir prefix
		rel := strings.TrimSuffix(path, k8sTmplSuffix)
		dstRel := strings.Replace(rel, k8sSourceDir, k8sOutputDir, 1)
		dstAbs := filepath.Join(opts.Root, filepath.FromSlash(dstRel))
		action, err := plannedFileAction(dstAbs, opts.Force)
		if err != nil {
			return fmt.Errorf("k8s: %w", err)
		}
		planned = append(planned, plannedFile{srcPath: path, dstPath: dstAbs, action: action})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("k8s: walk templates: %w", err)
	}

	if len(planned) == 0 {
		return nil, fmt.Errorf("k8s: no template files found under %s", k8sSourceDir)
	}

	// Sort for deterministic output order: base/ files before overlays/ files
	sort.Slice(planned, func(i, j int) bool {
		aBase := strings.Contains(planned[i].srcPath, "/base/")
		bBase := strings.Contains(planned[j].srcPath, "/base/")
		if aBase != bBase {
			return aBase
		}
		return planned[i].srcPath < planned[j].srcPath
	})

	writtenPaths := make([]string, 0, len(planned))
	filePlans := make([]PlanItem, 0, len(planned))
	var firstPath string

	for _, pf := range planned {
		body, err := renderK8sTemplate(pf.srcPath, data)
		if err != nil {
			return nil, err
		}
		writtenPaths = append(writtenPaths, pf.dstPath)
		if firstPath == "" {
			firstPath = pf.dstPath
		}
		filePlans = append(filePlans, PlanItem{Kind: "file", Action: pf.action, Path: pf.dstPath})
		if !opts.DryRun {
			if err := writeFile(pf.dstPath, body); err != nil {
				return nil, err
			}
		}
	}

	plan := append([]PlanItem(nil), filePlans...)
	plan = append(plan, PlanItem{Kind: "manifest", Action: "unchanged", Path: filepath.Join(".ncgo", "manifest.yaml"), Detail: "k8s files are project-level, not infra deps"})
	if opts.DryRun {
		plan = append(plan, PlanItem{Kind: "dry_run", Action: "info", Detail: "no files were written"})
	}

	next := []string{
		"review deploy/k8s/ files and customize image tags, resource limits, and replicas as needed",
		"kubectl kustomize deploy/k8s/overlays/dev | kubectl apply -f -  (apply dev overlay)",
		"kubectl kustomize deploy/k8s/overlays/prod | kubectl apply -f - (apply prod overlay)",
	}
	plan = append(plan, PlanItem{Kind: "next_step", Action: "run", Detail: next[0]})

	return &Result{
		WrittenPath:  firstPath,
		WrittenPaths: writtenPaths,
		NextSteps:    next,
		Plan:         plan,
		Updated:      false,
		DryRun:       opts.DryRun,
	}, nil
}

func renderK8sTemplate(srcPath string, data k8sTemplateData) ([]byte, error) {
	body, err := fs.ReadFile(assets.FS(), srcPath)
	if err != nil {
		return nil, fmt.Errorf("k8s: read embedded %s: %w", srcPath, err)
	}
	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("k8s: parse template %s: %w", srcPath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("k8s: execute template %s: %w", srcPath, err)
	}
	return buf.Bytes(), nil
}
