package audit

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	openshiftapi "github.com/openshift/api"
	configv1 "github.com/openshift/api/config/v1"
	assets "github.com/openshift/library-go/pkg/operator/apiserver/audit/bindata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/yaml"
)

var (
	basePolicy   auditv1.Policy
	profileRules = map[configv1.AuditProfileType][]auditv1.PolicyRule{}

	auditScheme         = runtime.NewScheme()
	auditCodecs         = serializer.NewCodecFactory(auditScheme)
	auditYamlSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, auditScheme, auditScheme)

	coreScheme         = runtime.NewScheme()
	coreCodecs         = serializer.NewCodecFactory(coreScheme)
	coreYamlSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, coreScheme, coreScheme)
)

func init() {
	if err := auditv1.AddToScheme(auditScheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(coreScheme); err != nil {
		panic(err)
	}

	bs, err := assets.Asset("pkg/operator/apiserver/audit/manifests/base-policy.yaml")
	if err != nil {
		panic(err)
	}
	if err := runtime.DecodeInto(coreCodecs.UniversalDecoder(auditv1.SchemeGroupVersion), bs, &basePolicy); err != nil {
		panic(err)
	}

	for _, profile := range []configv1.AuditProfileType{configv1.AuditProfileDefaultType, configv1.WriteRequestBodiesAuditProfileType, configv1.AllRequestBodiesAuditProfileType} {
		manifestName := fmt.Sprintf("%s-rules.yaml", strings.ToLower(string(profile)))
		bs, err := assets.Asset(path.Join("pkg/operator/apiserver/audit/manifests", manifestName))
		if err != nil {
			panic(err)
		}
		var rules []auditv1.PolicyRule
		if err := yaml.Unmarshal(bs, &rules); err != nil {
			panic(err)
		}
		profileRules[profile] = rules
	}
}

// DefaultPolicy brings back the default.yaml audit policy to init the api
func DefaultPolicy() ([]byte, error) {
	policy, err := GetAuditPolicy(configv1.Audit{Profile: configv1.AuditProfileDefaultType})
	if err != nil {
		return nil, fmt.Errorf("failed to retreive default audit policy: %v", err)
	}

	policy.Kind = "Policy"
	policy.APIVersion = auditv1.SchemeGroupVersion.String()

	var buf bytes.Buffer
	if err := auditYamlSerializer.Encode(policy, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetAuditPolicy computes the audit policy for the given audit config.
// Note: the returned Policy has Kind and APIVersion not set. This is responsibility of the caller
//       when serializing it.
func GetAuditPolicy(audit configv1.Audit, group []openshiftapi.CustomRule) (*auditv1.Policy, error) {
	p := basePolicy.DeepCopy()
	p.Name = string(audit.Profile)

	extraRules, ok := profileRules[audit.Profile]
	groupRule :=  openshiftapi.Audit.AuditCustomRule.group(""))
	if !ok {
		return nil, fmt.Errorf("unknown audit profile %q", audit.Profile)
	}
	p.Rules = append(p.Rules, groups)

	return p, nil
}
