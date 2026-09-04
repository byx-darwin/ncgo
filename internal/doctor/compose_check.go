package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
)

type composeFileApp struct {
	Build struct {
		Context    string `yaml:"context"`
		Dockerfile string `yaml:"dockerfile"`
	} `yaml:"build"`
}

type composeFile struct {
	Services map[string]composeFileApp `yaml:"services"`
}

// composeConsistencyChecks validates, for a micro workspace, that each
// service's go.mod local replace directives are reflected in compose.yaml's
// build.context/dockerfile and in the service's own Dockerfile COPY list.
func composeConsistencyChecks(root string, w *manifest.Workspace) []Check {
	var out []Check
	composePath := filepath.Join(root, "compose.yaml")
	var compose *composeFile

	for _, svc := range w.Services {
		serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
		replaces, err := shared.ParseLocalReplaces(serviceRoot)
		if err != nil {
			continue
		}
		siblings := shared.SiblingDirs(root, svc.Dir, replaces, w.Services)
		if len(siblings) == 0 {
			continue
		}
		if compose == nil {
			compose = loadComposeFile(composePath)
		}
		out = append(out, composeContextCheck(composePath, compose, svc, siblings))
		out = append(out, composeDockerfileCheck(serviceRoot, svc, siblings))
	}
	return out
}

func composeContextCheck(composePath string, compose *composeFile, svc manifest.WorkspaceService, siblings []string) Check {
	c := Check{ID: "compose.context." + svc.Name, Severity: SeverityError, File: composePath}
	wantContext := "."
	wantDockerfile := filepath.ToSlash(filepath.Join(svc.Dir, "Dockerfile"))
	if compose == nil {
		c.OK = false
		c.Message = fmt.Sprintf("compose.yaml not found or unreadable; service %s needs build.context=%q, build.dockerfile=%q for local replace dependency on %s", svc.Name, wantContext, wantDockerfile, strings.Join(siblings, ", "))
		c.Hint = "run `ncgo add infra` on the service (triggers a compose refresh) or re-run the scaffold"
		return c
	}
	app, ok := compose.Services[svc.Name]
	if !ok {
		c.OK = false
		c.Message = fmt.Sprintf("compose.yaml has no service entry for %s", svc.Name)
		c.Hint = "run `ncgo add infra` on the service (triggers a compose refresh) or edit compose.yaml directly"
		return c
	}
	if app.Build.Context != wantContext || app.Build.Dockerfile != wantDockerfile {
		c.OK = false
		c.Message = fmt.Sprintf("compose.yaml service %s has build.context=%q, build.dockerfile=%q; want build.context=%q, build.dockerfile=%q for local replace dependency on %s", svc.Name, app.Build.Context, app.Build.Dockerfile, wantContext, wantDockerfile, strings.Join(siblings, ", "))
		c.Hint = "run `ncgo add infra` on the service (triggers a compose refresh) or edit compose.yaml directly"
		return c
	}
	c.OK = true
	c.Message = fmt.Sprintf("compose.yaml service %s build context/dockerfile match its go.mod local replace deps", svc.Name)
	return c
}

func composeDockerfileCheck(serviceRoot string, svc manifest.WorkspaceService, siblings []string) Check {
	dockerfilePath := filepath.Join(serviceRoot, "Dockerfile")
	c := Check{ID: "compose.dockerfile." + svc.Name, Severity: SeverityError, File: dockerfilePath}
	body, err := os.ReadFile(dockerfilePath)
	if err != nil {
		c.OK = false
		c.Message = fmt.Sprintf("%s: %s", svc.Name, err.Error())
		c.Hint = "re-run the scaffold for this service or add infra to trigger a container-files refresh"
		return c
	}
	text := string(body)
	var missing []string
	for _, sibling := range siblings {
		sibling = filepath.ToSlash(sibling)
		if !strings.Contains(text, "COPY "+sibling+"/") {
			missing = append(missing, sibling)
		}
	}
	if len(missing) > 0 {
		c.OK = false
		c.Message = fmt.Sprintf("Dockerfile for %s is missing COPY line(s) for local replace dependency on %s", svc.Name, strings.Join(missing, ", "))
		c.Hint = "re-add the sibling COPY line(s) or re-run the scaffold to regenerate the Dockerfile"
		return c
	}
	c.OK = true
	c.Message = fmt.Sprintf("Dockerfile for %s COPYs all local replace dependencies", svc.Name)
	return c
}

func loadComposeFile(path string) *composeFile {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cf composeFile
	if err := yaml.Unmarshal(body, &cf); err != nil {
		return nil
	}
	return &cf
}
