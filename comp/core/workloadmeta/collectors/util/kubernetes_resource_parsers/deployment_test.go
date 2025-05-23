// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build kubeapiserver && test

package kubernetesresourceparsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
	"github.com/DataDog/datadog-agent/pkg/languagedetection/languagemodels"
)

func TestDeploymentParser_Parse(t *testing.T) {
	excludeAnnotations := []string{"ignore-annotation"}

	tests := []struct {
		name       string
		expected   *workloadmeta.KubernetesDeployment
		deployment *appsv1.Deployment
	}{
		{
			name: "everything",
			expected: &workloadmeta.KubernetesDeployment{
				EntityID: workloadmeta.EntityID{
					Kind: workloadmeta.KindKubernetesDeployment,
					ID:   "test-namespace/test-deployment",
				},
				Env:     "env",
				Service: "service",
				Version: "version",
				EntityMeta: workloadmeta.EntityMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label":                 "test-value",
						"tags.datadoghq.com/env":     "env",
						"tags.datadoghq.com/service": "service",
						"tags.datadoghq.com/version": "version",
					},
					Annotations: map[string]string{
						"internal.dd.datadoghq.com/nginx-cont.detected_langs":      "go,java,  python  ",
						"internal.dd.datadoghq.com/init.nginx-cont.detected_langs": "go,java,  python  ",
					},
				},
				InjectableLanguages: languagemodels.ContainersLanguages{
					*languagemodels.NewInitContainer("nginx-cont"): {
						languagemodels.Go:     {},
						languagemodels.Java:   {},
						languagemodels.Python: {},
					},
					*languagemodels.NewContainer("nginx-cont"): {
						languagemodels.Go:     {},
						languagemodels.Java:   {},
						languagemodels.Python: {},
					},
				},
			},

			deployment: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label":                 "test-value",
						"tags.datadoghq.com/env":     "env",
						"tags.datadoghq.com/service": "service",
						"tags.datadoghq.com/version": "version",
					},
					Annotations: map[string]string{
						"internal.dd.datadoghq.com/nginx-cont.detected_langs":      "go,java,  python  ",
						"internal.dd.datadoghq.com/init.nginx-cont.detected_langs": "go,java,  python  ",
						"ignore-annotation": "ignore",
					},
				},
			},
		},
		{
			name: "only usm",
			expected: &workloadmeta.KubernetesDeployment{
				EntityID: workloadmeta.EntityID{
					Kind: workloadmeta.KindKubernetesDeployment,
					ID:   "test-namespace/test-deployment",
				},
				EntityMeta: workloadmeta.EntityMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label":                 "test-value",
						"tags.datadoghq.com/env":     "env",
						"tags.datadoghq.com/service": "service",
						"tags.datadoghq.com/version": "version",
					},
					Annotations: map[string]string{},
				},
				Env:                 "env",
				Service:             "service",
				Version:             "version",
				InjectableLanguages: make(languagemodels.ContainersLanguages),
			},
			deployment: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label":                 "test-value",
						"tags.datadoghq.com/env":     "env",
						"tags.datadoghq.com/service": "service",
						"tags.datadoghq.com/version": "version",
					},
					Annotations: map[string]string{
						"ignore-annotation": "ignore",
					},
				},
			},
		},

		{
			name: "only languages",
			expected: &workloadmeta.KubernetesDeployment{
				EntityID: workloadmeta.EntityID{
					Kind: workloadmeta.KindKubernetesDeployment,
					ID:   "test-namespace/test-deployment",
				},
				EntityMeta: workloadmeta.EntityMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label": "test-value",
					},
					Annotations: map[string]string{
						"internal.dd.datadoghq.com/nginx-cont.detected_langs":      "go,java,  python  ",
						"internal.dd.datadoghq.com/init.nginx-cont.detected_langs": "go,java,  python  ",
					},
				},
				InjectableLanguages: languagemodels.ContainersLanguages{
					*languagemodels.NewInitContainer("nginx-cont"): {
						languagemodels.Go:     {},
						languagemodels.Java:   {},
						languagemodels.Python: {},
					},
					*languagemodels.NewContainer("nginx-cont"): {
						languagemodels.Go:     {},
						languagemodels.Java:   {},
						languagemodels.Python: {},
					},
				},
			},
			deployment: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label": "test-value",
					},
					Annotations: map[string]string{
						"ignore-annotation": "ignore",
						"internal.dd.datadoghq.com/nginx-cont.detected_langs":      "go,java,  python  ",
						"internal.dd.datadoghq.com/init.nginx-cont.detected_langs": "go,java,  python  ",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewDeploymentParser(excludeAnnotations)
			require.NoError(t, err)
			entity := parser.Parse(tt.deployment)
			storedDeployment, ok := entity.(*workloadmeta.KubernetesDeployment)
			require.True(t, ok)
			assert.Equal(t, tt.expected, storedDeployment)
		})
	}
}
