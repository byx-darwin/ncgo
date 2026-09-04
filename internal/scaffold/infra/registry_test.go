package infra

import "testing"

type fakePlugin struct {
	kind    string
	aliases []string
}

func (f fakePlugin) Kind() string                                       { return f.kind }
func (f fakePlugin) Aliases() []string                                  { return f.aliases }
func (f fakePlugin) ServiceScope() string                               { return "common" }
func (f fakePlugin) GoGetDeps() []string                                { return nil }
func (f fakePlugin) SetupSteps() []string                               { return nil }
func (f fakePlugin) HertzConfigKey() string                             { return "" }
func (f fakePlugin) AssetFiles(serviceKind string) ([]addOnFile, error) { return nil, nil }

func TestRegisterAndLookupByKindAndAlias(t *testing.T) {
	reg := newRegistry()
	reg.register(fakePlugin{kind: "widget", aliases: []string{"widget-alias"}})

	p, ok := reg.byKind("widget")
	if !ok || p.Kind() != "widget" {
		t.Fatalf("byKind(widget) = %v, %v; want widget plugin", p, ok)
	}
	p, ok = reg.byKind("widget-alias")
	if !ok || p.Kind() != "widget" {
		t.Fatalf("byKind(widget-alias) = %v, %v; want widget plugin via alias", p, ok)
	}
	if _, ok := reg.byKind("missing"); ok {
		t.Fatalf("byKind(missing) = ok=true; want ok=false")
	}
}

func TestRegisterDuplicateKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Kind() registration")
		}
	}()
	reg := newRegistry()
	reg.register(fakePlugin{kind: "widget"})
	reg.register(fakePlugin{kind: "widget"})
}

func TestRegisterDuplicateAliasPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate alias registration")
		}
	}()
	reg := newRegistry()
	reg.register(fakePlugin{kind: "a", aliases: []string{"shared"}})
	reg.register(fakePlugin{kind: "b", aliases: []string{"shared"}})
}
