package orchestrator

import (
	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
	"github.com/byx-darwin/ncgo/internal/scaffold/rulecenter"
)

// AddInfraOptions mirrors infra.Options for orchestrator callers.
type AddInfraOptions struct {
	Root   string
	Kind   string
	Force  bool
	Wire   bool
	DryRun bool
}

// AddInfraResult wraps infra.Add() output.
type AddInfraResult struct {
	Raw *infra.Result `json:"-"`
}

// RunAddInfra wraps infra.Add.
func RunAddInfra(opts AddInfraOptions) (*AddInfraResult, error) {
	res, err := infra.Add(infra.Options{
		Root:   opts.Root,
		Kind:   opts.Kind,
		Force:  opts.Force,
		Wire:   opts.Wire,
		DryRun: opts.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &AddInfraResult{Raw: res}, nil
}

// AddDomainOptions mirrors domain.Options for orchestrator callers.
type AddDomainOptions struct {
	Root   string
	Name   string
	Force  bool
	DryRun bool
}

// AddDomainResult wraps domain.Add() output.
type AddDomainResult struct {
	Raw *domain.Result `json:"-"`
}

// RunAddDomain wraps domain.Add.
func RunAddDomain(opts AddDomainOptions) (*AddDomainResult, error) {
	res, err := domain.Add(domain.Options{
		Root:   opts.Root,
		Name:   opts.Name,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &AddDomainResult{Raw: res}, nil
}

// AddMethodOptions mirrors method.Options for orchestrator callers.
type AddMethodOptions struct {
	Root  string
	Spec  string
	Layer string
}

// AddMethodResult wraps method.Add() output.
type AddMethodResult struct {
	Raw *method.Result `json:"-"`
}

// RunAddMethod wraps method.Add.
func RunAddMethod(opts AddMethodOptions) (*AddMethodResult, error) {
	res, err := method.Add(method.Options{
		Root:  opts.Root,
		Spec:  opts.Spec,
		Layer: opts.Layer,
	})
	if err != nil {
		return nil, err
	}
	return &AddMethodResult{Raw: res}, nil
}

// AddRuleCenterOptions mirrors rulecenter.Options for orchestrator callers.
type AddRuleCenterOptions struct {
	Root   string
	Addr   string
	Force  bool
	DryRun bool
}

// AddRuleCenterResult wraps rulecenter.Add() output.
type AddRuleCenterResult struct {
	Raw *rulecenter.Result `json:"-"`
}

// RunAddRuleCenter wraps rulecenter.Add.
func RunAddRuleCenter(opts AddRuleCenterOptions) (*AddRuleCenterResult, error) {
	res, err := rulecenter.Add(rulecenter.Options{
		Root:   opts.Root,
		Addr:   opts.Addr,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &AddRuleCenterResult{Raw: res}, nil
}
