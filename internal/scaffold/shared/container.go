package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/framework"
)

const (
	dockerConfigRelPath  = "conf/docker/conf.yaml"
	infraRedis           = "redis"
	infraKafka           = "kafka"
	infraES              = "es"
	infraClickHouse      = "clickhouse"
	infraRegistryPolaris = "registry_polaris"
	infraReleaseCanary   = "release_canary"
)

const serviceDockerIgnore = `.git/
.gitignore
.idea/
.vscode/
output/
tmp/
vendor/
template/
.ncgo/
scripts/
`

type composeApp struct {
	Name         string
	Kind         string
	Context      string
	Dockerfile   string // empty means the default "Dockerfile"
	HostPort     int
	WithDatabase bool
	Infra        []string
}

type composeFeatures struct {
	postgres bool
	redis    bool
	kafka    bool
	es       bool
	ch       bool
	nacos    bool
	polaris  bool
	vegeta   bool
}

// WriteServiceContainerFiles writes a generic multi-stage Dockerfile and
// .dockerignore for a mono/BFF/RPC service scaffold.
func WriteServiceContainerFiles(dir, kind string) error {
	port, err := servicePort(kind)
	if err != nil {
		return err
	}
	dockerfile := dockerfileTemplate("COPY . .\n", "", port)
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return fmt.Errorf("scaffold: write Dockerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte(serviceDockerIgnore), 0o644); err != nil {
		return fmt.Errorf("scaffold: write .dockerignore: %w", err)
	}
	return nil
}

// RewriteServiceDockerfileForSiblings overwrites dir/Dockerfile with a
// variant whose builder stage COPYs each sibling directory (workspace-root
// relative, e.g. "services/authority") plus the service's own directory,
// then sets WORKDIR to the service's own root-relative path before the
// build step — so `go build` resolves `../sibling` replace targets exactly
// as they resolve on disk. Call only when len(siblings) > 0.
func RewriteServiceDockerfileForSiblings(dir, kind, rootRel string, siblings []string) error {
	port, err := servicePort(kind)
	if err != nil {
		return err
	}
	rootRel = filepath.ToSlash(rootRel)
	copyLines := make([]string, 0, len(siblings)+1)
	for _, sibling := range siblings {
		sibling = filepath.ToSlash(sibling)
		copyLines = append(copyLines, fmt.Sprintf("COPY %s/ %s/\n", sibling, sibling))
	}
	copyLines = append(copyLines, fmt.Sprintf("COPY %s/ %s/\n", rootRel, rootRel))
	dockerfile := dockerfileTemplate(strings.Join(copyLines, ""), rootRel, port)
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return fmt.Errorf("scaffold: write Dockerfile: %w", err)
	}
	return nil
}

// dockerfileTemplate renders the shared multi-stage Dockerfile body.
// copyBlock is the builder-stage COPY instruction(s); workdir, if non-empty,
// is set (relative to /src) before the go build step runs.
func dockerfileTemplate(copyBlock, workdir string, port int) string {
	buildWorkdir := "/src"
	if workdir != "" {
		buildWorkdir = "/src/" + workdir
	}
	var workdirLine string
	if workdir != "" {
		workdirLine = fmt.Sprintf("WORKDIR %s\n", buildWorkdir)
	}
	return fmt.Sprintf(`FROM golang:1.26.5-alpine AS builder
WORKDIR /src

ENV CGO_ENABLED=0 GOOS=linux

%s%sRUN go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/app ./app

ENV GO_ENV=docker

EXPOSE %d

ENTRYPOINT ["./app"]
`, copyBlock, workdirLine, port)
}

// WriteServiceDockerConfig writes conf/docker/conf.yaml so containers can use
// compose service names while host-side development keeps using conf/dev.
func WriteServiceDockerConfig(dir string, m *manifest.Manifest) error {
	if m == nil {
		return fmt.Errorf("scaffold: write %s: nil manifest", filepath.FromSlash(dockerConfigRelPath))
	}
	body, err := renderServiceDockerConfig(m)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filepath.FromSlash(dockerConfigRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("scaffold: create %s dir: %w", filepath.FromSlash(dockerConfigRelPath), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", filepath.FromSlash(dockerConfigRelPath), err)
	}
	return nil
}

// WriteMonoCompose writes a compose.yaml for a mono/service scaffold.
func WriteMonoCompose(dir string, m *manifest.Manifest) error {
	if m == nil {
		return fmt.Errorf("scaffold: write compose.yaml: nil manifest")
	}
	port, err := servicePort(m.Service.Kind)
	if err != nil {
		return err
	}
	body, err := renderComposeProject(m.Service.Name, []composeApp{{
		Name:         m.Service.Name,
		Kind:         m.Service.Kind,
		Context:      ".",
		HostPort:     port,
		WithDatabase: m.Service.WithDatabase,
		Infra:        append([]string(nil), m.Infra...),
	}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write compose.yaml: %w", err)
	}
	return nil
}

// WriteWorkspaceCompose renders the micro-workspace compose.yaml based on the
// current ncgo.workspace service registry plus each service manifest when it is
// available.
func WriteWorkspaceCompose(root string, w *manifest.Workspace) error {
	if w == nil {
		return fmt.Errorf("scaffold: write compose.yaml: nil workspace")
	}
	apps, err := loadWorkspaceComposeApps(root, w)
	if err != nil {
		return err
	}
	body, err := renderComposeProject(w.Name, apps)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write compose.yaml: %w", err)
	}
	for _, app := range apps {
		if app.Context == "." {
			if err := ensureRootDockerIgnore(root); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// RefreshWorkspaceComposeForServiceRoot rewrites the parent workspace compose
// when root belongs to a listed service inside a micro workspace.
func RefreshWorkspaceComposeForServiceRoot(root string) error {
	workspaceRoot, w, _, err := findWorkspaceForServiceRoot(root)
	if err != nil || w == nil {
		return err
	}
	return WriteWorkspaceCompose(workspaceRoot, w)
}

// ensureRootDockerIgnore writes <root>/.dockerignore with the same exclude
// patterns as the per-service template, but only if the file does not
// already exist — it never overwrites user content.
func ensureRootDockerIgnore(root string) error {
	path := filepath.Join(root, ".dockerignore")
	if pathExists(path) {
		return nil
	}
	if err := os.WriteFile(path, []byte(serviceDockerIgnore), 0o644); err != nil {
		return fmt.Errorf("scaffold: write .dockerignore: %w", err)
	}
	return nil
}

func loadWorkspaceComposeApps(root string, w *manifest.Workspace) ([]composeApp, error) {
	if len(w.Services) == 0 {
		return nil, nil
	}
	services := append([]manifest.WorkspaceService(nil), w.Services...)
	sort.Slice(services, func(i, j int) bool {
		return services[i].Dir < services[j].Dir
	})
	hertzHostPort := 18080
	kitexHostPort := 18888
	apps := make([]composeApp, 0, len(services))
	for _, svc := range services {
		app := composeApp{
			Name:    svc.Name,
			Kind:    svc.Kind,
			Context: "./" + filepath.ToSlash(svc.Dir),
			Infra:   nil,
		}
		if svc.Kind == manifest.KindKitex {
			app.HostPort = kitexHostPort
			kitexHostPort++
		} else {
			app.HostPort = hertzHostPort
			hertzHostPort++
		}
		serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
		if pathExists(manifest.Path(serviceRoot)) {
			m, err := manifest.Load(serviceRoot)
			if err != nil {
				return nil, fmt.Errorf("scaffold: load workspace service %s: %w", svc.Name, err)
			}
			app.Name = m.Service.Name
			app.Kind = m.Service.Kind
			app.WithDatabase = m.Service.WithDatabase
			app.Infra = append([]string(nil), m.Infra...)
		}
		replaces, err := ParseLocalReplaces(serviceRoot)
		if err != nil {
			return nil, fmt.Errorf("scaffold: parse go.mod replace for %s: %w", svc.Name, err)
		}
		if siblings := SiblingDirs(root, svc.Dir, replaces, services); len(siblings) > 0 {
			app.Context = "."
			app.Dockerfile = filepath.ToSlash(filepath.Join(svc.Dir, "Dockerfile"))
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func renderComposeProject(projectName string, apps []composeApp) (string, error) {
	if len(apps) == 0 {
		return fmt.Sprintf("name: %s\nservices: {}\n", projectName), nil
	}
	features := composeFeatures{}
	for _, app := range apps {
		features.merge(composeFeaturesForApp(app))
	}
	var b strings.Builder
	b.WriteString("# Generated by ncgo for local development.\n")
	b.WriteString("# Containers use conf/docker/conf.yaml via GO_ENV=docker; host-side development keeps using conf/dev/conf.yaml.\n")
	if features.nacos || features.polaris {
		b.WriteString("# Optional config-center profiles:\n")
		if features.nacos {
			b.WriteString("#   docker compose --profile config-center-nacos up --build\n")
		}
		if features.polaris {
			b.WriteString("#   docker compose --profile config-center-polaris up --build\n")
		}
	}
	fmt.Fprintf(&b, "name: %s\nservices:\n", projectName)
	for _, app := range apps {
		if err := renderAppCompose(&b, app); err != nil {
			return "", err
		}
	}
	if features.postgres {
		renderPostgresCompose(&b)
	}
	if features.redis {
		renderRedisCompose(&b)
	}
	if features.kafka {
		renderKafkaCompose(&b)
	}
	if features.es {
		renderESCompose(&b)
	}
	if features.ch {
		renderClickHouseCompose(&b)
	}
	if features.nacos {
		renderNacosCompose(&b)
	}
	if features.polaris {
		renderPolarisCompose(&b)
	}
	if features.vegeta {
		renderVegetaCompose(&b)
	}
	volumes := composeVolumeNames(features)
	if len(volumes) > 0 {
		b.WriteString("volumes:\n")
		for _, volume := range volumes {
			fmt.Fprintf(&b, "  %s:\n", volume)
		}
	}
	return b.String(), nil
}

func renderAppCompose(b *strings.Builder, app composeApp) error {
	containerPort, err := servicePort(app.Kind)
	if err != nil {
		return err
	}
	features := composeFeaturesForApp(app)
	fmt.Fprintf(b, "  %s:\n", app.Name)
	dockerfile := app.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	b.WriteString("    build:\n")
	fmt.Fprintf(b, "      context: %s\n", app.Context)
	fmt.Fprintf(b, "      dockerfile: %s\n", dockerfile)
	b.WriteString("    environment:\n")
	b.WriteString("      GO_ENV: docker\n")
	if app.WithDatabase {
		fmt.Fprintf(b, "      DATABASE_URL: postgres://postgres:postgres@postgres:5432/%s?sslmode=disable\n", app.Name)
	}
	deps := dependencyServiceNames(features)
	if len(deps) > 0 {
		b.WriteString("    depends_on:\n")
		for _, dep := range deps {
			fmt.Fprintf(b, "      - %s\n", dep)
		}
	}
	b.WriteString("    ports:\n")
	fmt.Fprintf(b, "      - \"%d:%d\"\n", app.HostPort, containerPort)
	return nil
}

func renderPostgresCompose(b *strings.Builder) {
	b.WriteString("  postgres:\n")
	b.WriteString("    image: postgres:alpine\n")
	b.WriteString("    environment:\n")
	b.WriteString("      POSTGRES_DB: app\n")
	b.WriteString("      POSTGRES_USER: postgres\n")
	b.WriteString("      POSTGRES_PASSWORD: postgres\n")
	b.WriteString("      PGDATA: /var/lib/postgresql/data/pgdata\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"5432:5432\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - postgres-data:/var/lib/postgresql/data\n")
}

func renderRedisCompose(b *strings.Builder) {
	b.WriteString("  redis:\n")
	b.WriteString("    image: redis:7-alpine\n")
	b.WriteString("    command: [\"redis-server\", \"--appendonly\", \"yes\"]\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"6379:6379\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - redis-data:/data\n")
}

func renderKafkaCompose(b *strings.Builder) {
	b.WriteString("  kafka:\n")
	b.WriteString("    image: docker.io/bitnami/kafka:4.2\n")
	b.WriteString("    environment:\n")
	b.WriteString("      KAFKA_ENABLE_KRAFT: \"yes\"\n")
	b.WriteString("      KAFKA_CFG_NODE_ID: \"1\"\n")
	b.WriteString("      KAFKA_CFG_PROCESS_ROLES: controller,broker\n")
	b.WriteString("      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER\n")
	b.WriteString("      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:9094\n")
	b.WriteString("      KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT\n")
	b.WriteString("      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092,EXTERNAL://localhost:9094\n")
	b.WriteString("      KAFKA_CFG_INTER_BROKER_LISTENER_NAME: PLAINTEXT\n")
	b.WriteString("      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093\n")
	b.WriteString("      KAFKA_KRAFT_CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk\n")
	b.WriteString("      ALLOW_PLAINTEXT_LISTENER: \"yes\"\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"9092:9092\"\n")
	b.WriteString("      - \"9094:9094\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - kafka-data:/bitnami/kafka\n")
}

func renderESCompose(b *strings.Builder) {
	b.WriteString("  elasticsearch:\n")
	b.WriteString("    image: docker.elastic.co/elasticsearch/elasticsearch:8.14.3\n")
	b.WriteString("    environment:\n")
	b.WriteString("      discovery.type: single-node\n")
	b.WriteString("      xpack.security.enabled: \"false\"\n")
	b.WriteString("      ES_JAVA_OPTS: -Xms512m -Xmx512m\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"9200:9200\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - elasticsearch-data:/usr/share/elasticsearch/data\n")
}

func renderClickHouseCompose(b *strings.Builder) {
	b.WriteString("  clickhouse:\n")
	b.WriteString("    image: clickhouse/clickhouse-server:24.3\n")
	b.WriteString("    environment:\n")
	b.WriteString("      CLICKHOUSE_DB: default\n")
	b.WriteString("      CLICKHOUSE_USER: default\n")
	b.WriteString("      CLICKHOUSE_PASSWORD: \"\"\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"8123:8123\"\n")
	b.WriteString("      - \"9000:9000\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - clickhouse-data:/var/lib/clickhouse\n")
}

func renderNacosCompose(b *strings.Builder) {
	b.WriteString("  nacos:\n")
	b.WriteString("    image: nacos/nacos-server:v2.5.2\n")
	b.WriteString("    profiles:\n")
	b.WriteString("      - config-center-nacos\n")
	b.WriteString("    environment:\n")
	b.WriteString("      MODE: standalone\n")
	b.WriteString("      NACOS_AUTH_ENABLE: \"false\"\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"8848:8848\"\n")
	b.WriteString("      - \"9848:9848\"\n")
}

func renderPolarisCompose(b *strings.Builder) {
	b.WriteString("  polaris:\n")
	b.WriteString("    image: polarismesh/polaris-server-standalone:v1.18.1\n")
	b.WriteString("    profiles:\n")
	b.WriteString("      - config-center-polaris\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"8080:8080\"\n")
	b.WriteString("      - \"8090:8090\"\n")
	b.WriteString("      - \"8091:8091\"\n")
	b.WriteString("      - \"8093:8093\"\n")
	b.WriteString("      - \"8761:8761\"\n")
	b.WriteString("      - \"9000:9000\"\n")
	b.WriteString("      - \"9090:9090\"\n")
	b.WriteString("      - \"15010:15010\"\n")
}

func renderVegetaCompose(b *strings.Builder) {
	b.WriteString("  vegeta:\n")
	b.WriteString("    build:\n")
	b.WriteString("      context: .\n")
	b.WriteString("      dockerfile: Dockerfile.vegeta\n")
	b.WriteString("    entrypoint: [\"/bin/sh\"]\n")
}

// WriteVegetaDockerfile writes a Dockerfile.vegeta template for building the
// vegeta load testing tool. It is only called for Hertz services that have
// postgres enabled (rate-limit E2E testing).
func WriteVegetaDockerfile(dir string) error {
	const content = `FROM golang:1.26.5-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=0 go install github.com/tsenart/vegeta/v12@latest

FROM alpine:3.20
COPY --from=builder /go/bin/vegeta /usr/local/bin/vegeta
ENTRYPOINT ["vegeta"]
`
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile.vegeta"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("scaffold: write Dockerfile.vegeta: %w", err)
	}
	return nil
}

func renderServiceDockerConfig(m *manifest.Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("scaffold: render docker config: nil manifest")
	}
	kind := m.Service.Kind
	if kind == "" {
		kind = manifest.KindHertz
	}
	adapter, ok := framework.Get(kind)
	if !ok {
		return "", fmt.Errorf("scaffold: unsupported service kind %q for docker config", m.Service.Kind)
	}
	blocks := []string{"# Generated by ncgo for docker-compose based local development.\nenv: docker"}
	blocks = append(blocks, adapter.DockerConfigBlocks(m)...)
	return strings.Join(blocks, "\n\n") + "\n", nil
}

func composeFeaturesForApp(app composeApp) composeFeatures {
	features := composeFeatures{postgres: app.WithDatabase}
	for _, kind := range app.Infra {
		switch kind {
		case infraRedis:
			features.redis = true
		case infraKafka:
			features.kafka = true
		case infraES:
			features.es = true
		case infraClickHouse:
			features.ch = true
		case infraRegistryPolaris:
			features.polaris = true
		case infraReleaseCanary:
			features.nacos = true
			features.polaris = true
		}
	}
	appKind := app.Kind
	if appKind == "" {
		appKind = manifest.KindHertz
	}
	if adapter, ok := framework.Get(appKind); ok {
		flags := adapter.ComposeFeatures(app.WithDatabase)
		features.nacos = features.nacos || flags.Nacos
		features.polaris = features.polaris || flags.Polaris
		features.vegeta = features.vegeta || flags.Vegeta
	}
	return features
}

func dependencyServiceNames(features composeFeatures) []string {
	deps := make([]string, 0, 6)
	if features.postgres {
		deps = append(deps, "postgres")
	}
	if features.redis {
		deps = append(deps, "redis")
	}
	if features.kafka {
		deps = append(deps, "kafka")
	}
	if features.es {
		deps = append(deps, "elasticsearch")
	}
	if features.ch {
		deps = append(deps, "clickhouse")
	}
	return deps
}

func composeVolumeNames(features composeFeatures) []string {
	volumes := make([]string, 0, 6)
	if features.postgres {
		volumes = append(volumes, "postgres-data")
	}
	if features.redis {
		volumes = append(volumes, "redis-data")
	}
	if features.kafka {
		volumes = append(volumes, "kafka-data")
	}
	if features.es {
		volumes = append(volumes, "elasticsearch-data")
	}
	if features.ch {
		volumes = append(volumes, "clickhouse-data")
	}
	return volumes
}

func (f *composeFeatures) merge(other composeFeatures) {
	f.postgres = f.postgres || other.postgres
	f.redis = f.redis || other.redis
	f.kafka = f.kafka || other.kafka
	f.es = f.es || other.es
	f.ch = f.ch || other.ch
	f.nacos = f.nacos || other.nacos
	f.polaris = f.polaris || other.polaris
	f.vegeta = f.vegeta || other.vegeta
}

func findWorkspaceForServiceRoot(root string) (string, *manifest.Workspace, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, "", fmt.Errorf("scaffold: resolve root %s: %w", root, err)
	}
	for dir := filepath.Dir(absRoot); ; {
		workspacePath := manifest.WorkspacePath(dir)
		if pathExists(workspacePath) {
			w, err := manifest.LoadWorkspace(dir)
			if err != nil {
				return "", nil, "", fmt.Errorf("scaffold: load parent workspace %s: %w", dir, err)
			}
			serviceRel, err := filepath.Rel(dir, absRoot)
			if err != nil {
				return "", nil, "", fmt.Errorf("scaffold: relate %s to %s: %w", dir, absRoot, err)
			}
			serviceRel = filepath.ToSlash(filepath.Clean(serviceRel))
			for _, svc := range w.Services {
				if filepath.ToSlash(filepath.Clean(svc.Dir)) == serviceRel {
					return dir, w, serviceRel, nil
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, "", nil
		}
		dir = parent
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func servicePort(kind string) (int, error) {
	if kind == "" {
		kind = manifest.KindHertz
	}
	adapter, ok := framework.Get(kind)
	if !ok {
		return 0, fmt.Errorf("scaffold: unsupported service kind %q for container files", kind)
	}
	return adapter.ContainerPort(), nil
}
