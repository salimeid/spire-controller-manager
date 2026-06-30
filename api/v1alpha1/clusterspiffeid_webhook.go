/*
Copyright 2021 SPIRE Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	dnsNameTemplateName          = "dnsNameTemplate"
	spiffeIDTemplateName         = "spiffeIDTemplate"
	workloadSelectorTemplateName = "workloadSelectorTemplate"
)

// FederatesWithPatternMetacharacters are the path.Match metacharacters that mark
// a federatesWith value as a pattern rather than a literal trust domain.
const FederatesWithPatternMetacharacters = "*?[]"

// log is for logging in this package.
var clusterspiffeidlog = logf.Log.WithName("clusterspiffeid-resource")

func (r *ClusterSPIFFEID) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(&ClusterSPIFFEIDCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-spire-spiffe-io-v1alpha1-clusterspiffeid,mutating=false,failurePolicy=fail,sideEffects=None,groups=spire.spiffe.io,resources=clusterspiffeids,verbs=create;update,versions=v1alpha1,name=vclusterspiffeid.kb.io,admissionReviewVersions=v1

type ClusterSPIFFEIDCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ admission.Validator[*ClusterSPIFFEID] = &ClusterSPIFFEIDCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateCreate(_ context.Context, obj *ClusterSPIFFEID) (admission.Warnings, error) {
	clusterspiffeidlog.Info("validate create", "name", obj.Name)
	return r.validate(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateUpdate(_ context.Context, _ *ClusterSPIFFEID, nobj *ClusterSPIFFEID) (admission.Warnings, error) {
	clusterspiffeidlog.Info("validate update", "name", nobj.Name)
	return r.validate(nobj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateDelete(context.Context, *ClusterSPIFFEID) (admission.Warnings, error) {
	// Deletes are not validated.
	return nil, nil
}

func (r *ClusterSPIFFEIDCustomValidator) validate(o *ClusterSPIFFEID) (admission.Warnings, error) {
	_, err := ParseClusterSPIFFEIDSpec(&o.Spec)
	return nil, err
}

// +kubebuilder:object:generate=false
// ParsedClusterSPIFFEIDSpec is a parsed and validated ClusterSPIFFEIDSpec
type ParsedClusterSPIFFEIDSpec struct {
	SPIFFEIDTemplate          *template.Template
	NamespaceSelector         labels.Selector
	PodSelector               labels.Selector
	TTL                       time.Duration
	JWTTTL                    time.Duration
	FederatesWith             []spiffeid.TrustDomain
	DNSNameTemplates          []*template.Template
	WorkloadSelectorTemplates []*template.Template
	Admin                     bool
	Downstream                bool
	AutoPopulateDNSNames      bool
	Hint                      string
}

// ParseClusterSPIFFEIDSpec parses and validates the fields in the ClusterSPIFFEIDSpec
func ParseClusterSPIFFEIDSpec(spec *ClusterSPIFFEIDSpec) (*ParsedClusterSPIFFEIDSpec, error) {
	return parseClusterSPIFFEIDSpec(spec, false)
}

// ParseClusterSPIFFEIDSpecWithPatternExpansion parses and validates the fields
// in the ClusterSPIFFEIDSpec, allowing pattern values in federatesWith.
func ParseClusterSPIFFEIDSpecWithPatternExpansion(spec *ClusterSPIFFEIDSpec) (*ParsedClusterSPIFFEIDSpec, error) {
	return parseClusterSPIFFEIDSpec(spec, true)
}

func parseClusterSPIFFEIDSpec(spec *ClusterSPIFFEIDSpec, enableFederatesWithPatternExpansion bool) (*ParsedClusterSPIFFEIDSpec, error) {
	if spec.SPIFFEIDTemplate == "" {
		return nil, errors.New("empty SPIFFEID template")
	}

	spiffeIDTemplate, err := template.New(spiffeIDTemplateName).Parse(spec.SPIFFEIDTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFEID template: %w", err)
	}

	var namespaceSelector labels.Selector
	if spec.NamespaceSelector != nil {
		namespaceSelector, err = metav1.LabelSelectorAsSelector(spec.NamespaceSelector)
		if err != nil {
			return nil, err
		}
	}

	var podSelector labels.Selector
	if spec.PodSelector != nil {
		podSelector, err = metav1.LabelSelectorAsSelector(spec.PodSelector)
		if err != nil {
			return nil, err
		}
	}

	federatesWith := make([]spiffeid.TrustDomain, 0, len(spec.FederatesWith))
	for _, value := range spec.FederatesWith {
		td, err := spiffeid.TrustDomainFromString(value)
		if err == nil {
			federatesWith = append(federatesWith, td)
			continue
		}
		// Not a literal trust domain. When pattern expansion is enabled, accept
		// it if it is a valid pattern that could expand to a valid trust domain;
		// the matching domains are resolved later against known trust domains.
		if !enableFederatesWithPatternExpansion {
			return nil, fmt.Errorf("invalid federatesWith value: %w", err)
		}
		if err := validateFederatesWithPattern(value); err != nil {
			return nil, err
		}
	}

	var dnsNameTemplates []*template.Template
	for _, value := range spec.DNSNameTemplates {
		dnsNameTemplate, err := template.New(dnsNameTemplateName).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid dnsNameTemplate value: %w", err)
		}
		dnsNameTemplates = append(dnsNameTemplates, dnsNameTemplate)
	}

	var workloadSelectorTemplates []*template.Template
	for _, value := range spec.WorkloadSelectorTemplates {
		workloadSelectorTemplate, err := template.New(workloadSelectorTemplateName).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid workloadSelectorTemplates value: %w", err)
		}
		workloadSelectorTemplates = append(workloadSelectorTemplates, workloadSelectorTemplate)
	}

	return &ParsedClusterSPIFFEIDSpec{
		SPIFFEIDTemplate:          spiffeIDTemplate,
		NamespaceSelector:         namespaceSelector,
		PodSelector:               podSelector,
		TTL:                       spec.TTL.Duration,
		JWTTTL:                    spec.JWTTTL.Duration,
		FederatesWith:             federatesWith,
		DNSNameTemplates:          dnsNameTemplates,
		WorkloadSelectorTemplates: workloadSelectorTemplates,
		Admin:                     spec.Admin,
		Downstream:                spec.Downstream,
		AutoPopulateDNSNames:      spec.AutoPopulateDNSNames,
		Hint:                      spec.Hint,
	}, nil
}

// patternMetacharStripper removes path.Match metacharacters so the remaining
// literal skeleton of a federatesWith pattern can be validated as a trust domain.
var patternMetacharStripper = newMetacharStripper(FederatesWithPatternMetacharacters)

func newMetacharStripper(metachars string) *strings.Replacer {
	pairs := make([]string, 0, len(metachars)*2)
	for _, c := range metachars {
		pairs = append(pairs, string(c), "")
	}
	return strings.NewReplacer(pairs...)
}

// validateFederatesWithPattern reports whether value is a federatesWith pattern
// that could expand to a valid trust domain. It rejects malformed glob syntax and
// patterns whose literal characters are not valid trust domain characters.
func validateFederatesWithPattern(value string) error {
	if _, err := path.Match(value, ""); err != nil {
		return fmt.Errorf("invalid federatesWith pattern: %w", err)
	}
	skeleton := patternMetacharStripper.Replace(value)
	if skeleton == "" {
		// Pattern is composed solely of wildcards; it can match a valid trust domain.
		return nil
	}
	if _, err := spiffeid.TrustDomainFromString(skeleton); err != nil {
		return fmt.Errorf("federatesWith pattern %q cannot expand to a valid trust domain: %w", value, err)
	}
	return nil
}
