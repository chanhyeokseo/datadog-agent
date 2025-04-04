// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build sds

//nolint:revive
package sds

import (
	"bytes"
	"testing"
	"time"

	sds "github.com/DataDog/dd-sensitive-data-scanner/sds-go/go"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/pkg/logs/message"
)

func TestCreateScanner(t *testing.T) {
	require := require.New(t)

	standardRules := []byte(`
        {"priority":1,"is_enabled":true,"rules":[
            {
                "id":"zero-0",
                "description":"zero desc",
                "name":"zero",
                "definitions": [{"version":1, "pattern":"zero"}]
            },{
                "id":"one-1",
                "description":"one desc",
                "name":"one",
                "definitions": [{"version":1, "pattern":"one"}]
            },{
                "id":"two-2",
                "description":"two desc",
                "name":"two",
                "definitions": [{"version":1, "pattern":"two"}]
            }
        ]}
    `)
	agentConfig := []byte(`
        {"is_enabled":true,"rules":[
            {
                "id": "random000",
                "name":"zero",
                "definition":{"standard_rule_id":"zero-0"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":false
            },{
                "id": "random111",
                "name":"one",
                "definition":{"standard_rule_id":"one-1"},
                "match_action":{"type":"Hash"},
                "is_enabled":false
            },{
                "id": "random222",
                "name":"two",
                "definition":{"standard_rule_id":"two-2"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":false
            }
        ]}
    `)

	// scanner creation
	// -----

	s := CreateScanner("")

	require.NotNil(s, "the scanner should not be nil after a creation")

	isActive, err := s.Reconfigure(ReconfigureOrder{
		Type:   StandardRules,
		Config: standardRules,
	})

	require.NoError(err, "configuring the standard rules should not fail")
	require.False(isActive, "with only standard rules, the scanner can't be active")

	// now that we have some definitions, we can configure the scanner
	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "this one shouldn't fail, all rules are disabled but it's OK as long as there are no rules in the scanner")

	require.NotNil(s, "the scanner should not become a nil object")
	require.False(isActive, "all rules are disabled, the scanner should not be active")

	if s != nil && len(s.configuredRules) > 0 {
		t.Errorf("No rules should be configured, they're all disabled. Got (%v) rules configured instead.", len(s.configuredRules))
	}

	// enable 2 of the 3 rules
	// ------

	agentConfig = bytes.Replace(agentConfig, []byte("\"is_enabled\":false"), []byte("\"is_enabled\":true"), 2)

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "this one should not fail since two rules are enabled: %v", err)
	require.True(isActive, "the scanner should now be active")

	require.NotNil(s.Scanner, "the Scanner should've been created, it should not be nil")
	require.NotNil(s.Scanner.RuleConfigs, "the Scanner should use rules")

	require.Len(s.Scanner.RuleConfigs, 2, "the Scanner should use two rules")
	require.Len(s.configuredRules, 2, "only two rules should be part of this scanner.")

	// order matters, it's ok to test rules by [] access
	require.Equal(s.configuredRules[0].Name, "zero", "incorrect rules selected for configuration")
	require.Equal(s.configuredRules[1].Name, "one", "incorrect rules selected for configuration")

	// compare rules returned by GetRuleByIdx

	zero, err := s.GetRuleByIdx(0)
	require.NoError(err, "GetRuleByIdx on 0 should not fail")
	require.Equal(s.configuredRules[0].ID, zero.ID, "incorrect rule returned")
	one, err := s.GetRuleByIdx(1)
	require.NoError(err, "GetRuleByIdx on 1 should not fail")
	require.Equal(s.configuredRules[1].ID, one.ID, "incorrect rule returned")

	// disable the rule zero
	// only "one" is left enabled
	// -----

	agentConfig = []byte(`
        {"is_enabled":true,"rules":[
            {
                "id": "random000",
                "name":"zero",
                "definition":{"standard_rule_id":"zero-0"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":false
            },{
                "id": "random111",
                "name":"one",
                "definition":{"standard_rule_id":"one-1"},
                "match_action":{"type":"Hash"},
                "is_enabled":true
            },{
                "id": "random222",
                "name":"two",
                "definition":{"standard_rule_id":"two-2"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":false
            }
        ]}
    `)

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "this one should not fail since one rule is enabled")
	require.Len(s.configuredRules, 1, "only one rules should be part of this scanner")
	require.True(isActive, "the scanner should be active as one rule is enabled")

	// order matters, it's ok to test rules by [] access
	require.Equal(s.configuredRules[0].Name, "one", "incorrect rule selected for configuration")

	rule, err := s.GetRuleByIdx(0)
	require.NoError(err, "incorrect rule returned")
	require.Equal(rule.ID, s.configuredRules[0].ID, "the scanner hasn't been configured with the good rule")
	require.Equal(rule.Name, "one", "the scanner hasn't been configured with the good rule")

	// disable the whole group

	agentConfig = []byte(`
        {"is_enabled":false,"rules":[
            {
                "id": "random000",
                "name":"zero",
                "definition":{"standard_rule_id":"zero-0"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":true
            },{
                "id": "random111",
                "name":"one",
                "definition":{"standard_rule_id":"one-1"},
                "match_action":{"type":"Hash"},
                "is_enabled":true
            },{
                "id": "random222",
                "name":"two",
                "definition":{"standard_rule_id":"two-2"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":false
            }
        ]}
    `)

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "no error should happen")
	require.Len(s.configuredRules, 0, "The group is disabled, no rules should be configured.")
	require.False(isActive, "the scanner should've been disabled")
}

// TestEmptyConfiguration validates that the scanner is destroyed when receiving
// an empty configuration.
func TestEmptyConfiguration(t *testing.T) {
	require := require.New(t)

	standardRules := []byte(`
        {"priority":1,"is_enabled":true,"rules":[
            {
                "id":"zero-0",
                "description":"zero desc",
                "name":"zero",
                "definitions": [{"version":1, "pattern":"zero"}]
            },{
                "id":"one-1",
                "description":"one desc",
                "name":"one",
                "definitions": [{"version":1, "pattern":"one"}]
            },{
                "id":"two-2",
                "description":"two desc",
                "name":"two",
                "definitions": [{"version":1, "pattern":"two"}]
            }
        ]}
    `)
	agentConfig := []byte(`
        {"is_enabled":true,"rules":[
            {
                "id": "random000",
                "name":"zero",
                "definition":{"standard_rule_id":"zero-0"},
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":true
            }
        ]}
    `)

	s := CreateScanner("")

	require.NotNil(s, "the scanner should not be nil after a creation")

	isActive, err := s.Reconfigure(ReconfigureOrder{
		Type:   StandardRules,
		Config: standardRules,
	})

	require.NoError(err, "configuring the standard rules should not fail")
	require.False(isActive, "with only standard rules, the scanner can't be active")

	// configure with one rule

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "this one should not fail since one rule is enabled")
	require.Len(s.configuredRules, 1, "only one rules should be part of this scanner")
	require.True(isActive, "one rule is enabled, the scanner should be active")
	require.NotNil(s.Scanner)

	// empty reconfiguration

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: []byte("{}"),
	})

	require.NoError(err)
	require.Len(s.configuredRules, 0)
	require.Nil(s.Scanner)
	require.False(isActive, "no active rule, the scanner should be disabled")

	// re-enabling with on rule

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.NoError(err, "this one should not fail since one rule is enabled")
	require.Len(s.configuredRules, 1, "only one rules should be part of this scanner")
	require.True(isActive, "one rule is enabled, the scanner should be active")
	require.NotNil(s.Scanner)

	// the StopProcessing signal

	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   StopProcessing,
		Config: nil,
	})

	require.NoError(err)
	require.Len(s.configuredRules, 0)
	require.Nil(s.Scanner)
	require.False(isActive, "no active rule, the scanner should be disabled")
}

func TestIsReady(t *testing.T) {
	require := require.New(t)

	standardRules := []byte(`
        {"priority":1,"rules":[
            {
                "id":"zero-0",
                "description":"zero desc",
                "name":"zero",
                "definitions": [{"version":1, "pattern":"zero"}]
            },{
                "id":"one-1",
                "description":"one desc",
                "name":"one",
                "definitions": [{"version":1, "pattern":"one"}]
            },{
                "id":"two-2",
                "description":"two desc",
                "name":"two",
                "definitions": [{"version":1, "pattern":"two"}]
            }
        ]}
    `)
	agentConfig := []byte(`
        {"is_enabled":true,"rules":[
            {
                "id":"random-0000000",
                "definition":{"standard_rule_id":"zero-0"},
                "name":"zero",
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":true
            },{
                "id":"random-111",
                "definition":{"standard_rule_id":"one-1"},
                "name":"one",
                "match_action":{"type":"Hash"},
                "is_enabled":true
            }
        ]}
    `)

	// scanner creation
	// -----

	s := CreateScanner("")

	require.NotNil(s, "the scanner should not be nil after a creation")
	require.False(s.IsReady(), "at this stage, the scanner should not be considered ready, no definitions received")

	isActive, err := s.Reconfigure(ReconfigureOrder{
		Type:   StandardRules,
		Config: standardRules,
	})

	require.NoError(err, "configuring the definitions should not fail")
	require.False(s.IsReady(), "at this stage, the scanner should not be considered ready, no user config received")
	require.False(isActive, "only standard rules configured, the scanner should not be active")

	// now that we have some definitions, we can configure the scanner
	isActive, err = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.True(s.IsReady(), "at this stage, the scanner should be considered ready")
	require.True(isActive, "the scanner has some enabled rules, it should be active")
}

// TestScan validates that everything fits and works. It's not validating
// the scanning feature itself, which is done in the library.
func TestScan(t *testing.T) {
	require := require.New(t)

	standardRules := []byte(`
        {"priority":1,"rules":[
            {
                "id":"zero-0",
                "description":"zero desc",
                "name":"zero",
                "definitions": [{"version":1, "pattern":"zero"}]
            },{
                "id":"one-1",
                "description":"one desc",
                "name":"one",
                "definitions": [{"version":1, "pattern":"one"}]
            },{
                "id":"two-2",
                "description":"two desc",
                "name":"two",
                "definitions": [{"version":1, "pattern":"two"}]
            }
        ]}
    `)
	agentConfig := []byte(`
        {"is_enabled":true,"rules":[
            {
                "id":"random-00000",
                "definition":{"standard_rule_id":"zero-0"},
                "name":"zero",
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":true
            },{
                "id":"random-11111",
                "definition":{"standard_rule_id":"one-1"},
                "name":"one",
                "match_action":{"type":"Redact","placeholder":"[REDACTED]"},
                "is_enabled":true
            }
        ]}
    `)

	// scanner creation
	// -----

	s := CreateScanner("")
	require.NotNil(s, "the returned scanner should not be nil")

	isActive, _ := s.Reconfigure(ReconfigureOrder{
		Type:   StandardRules,
		Config: standardRules,
	})

	require.False(isActive, "only standard rules, the scanner should be disabled")

	isActive, _ = s.Reconfigure(ReconfigureOrder{
		Type:   AgentConfig,
		Config: agentConfig,
	})

	require.True(s.IsReady(), "at this stage, the scanner should be considered ready")
	require.True(isActive, "rules are configured, the scanner should be active")

	type result struct {
		matched    bool
		event      string
		matchCount int
	}

	tests := map[string]result{
		"one two three go!": {
			matched:    true,
			event:      "[REDACTED] two three go!",
			matchCount: 1,
		},
		"after zero comes one, after one comes two, and the rest is history": {
			matched:    true,
			event:      "after [redacted] comes [REDACTED], after [REDACTED] comes two, and the rest is history",
			matchCount: 3,
		},
		"and so we go": {
			matched:    false,
			event:      "and so we go",
			matchCount: 0,
		},
	}

	for k, v := range tests {
		msg := message.Message{}
		matched, processed, err := s.Scan([]byte(k), &msg)
		require.NoError(err, "scanning these event should not fail.")
		require.False(matched && processed == nil, "incorrect result: nil processed event returned")
		require.Equal(matched, v.matched, "unexpected match/non-match")
		require.Equal(string(processed), v.event, "incorrect result")
	}
}

// TestCloseCycleScan validates that the close cycle works well (not blocking, not racing).
// by trying hard to reproduce a possible race on close.
func TestCloseCycleScan(t *testing.T) {
	require := require.New(t)

	standardRules := []byte(`
        {"priority":1,"rules":[
            {
                "id":"zero-0",
                "description":"zero desc",
                "name":"zero",
                "definitions": [{"version":1, "pattern":"zero"}]
            }
        ]}
    `)
	agentConfig := []byte(`
        {"is_enabled":true,"rules":[
            {
                "id":"random-00000",
                "definition":{"standard_rule_id":"zero-0"},
                "name":"zero",
                "match_action":{"type":"Redact","placeholder":"[redacted]"},
                "is_enabled":true
            },{
                "id":"random-11111",
                "definition":{"standard_rule_id":"zero-0"},
                "name":"one",
                "match_action":{"type":"Redact","placeholder":"[REDACTED]"},
                "is_enabled":true
            }
        ]}
    `)

	// scanner creation
	// -----

	for i := 0; i < 10; i++ {
		s := CreateScanner("")
		require.NotNil(s, "the returned scanner should not be nil")

		_, _ = s.Reconfigure(ReconfigureOrder{
			Type:   StandardRules,
			Config: standardRules,
		})
		isActive, _ := s.Reconfigure(ReconfigureOrder{
			Type:   AgentConfig,
			Config: agentConfig,
		})

		require.True(s.IsReady(), "at this stage, the scanner should be considered ready")
		require.True(isActive, "the scanner should be active")

		type result struct {
			matched    bool
			event      string
			matchCount int
		}

		tests := map[string]result{
			"one two three go!": {
				matched:    true,
				event:      "[REDACTED] two three go!",
				matchCount: 1,
			},
			"after zero comes one, after one comes two, and the rest is history": {
				matched:    true,
				event:      "after [redacted] comes [REDACTED], after [REDACTED] comes two, and the rest is history",
				matchCount: 3,
			},
			"and so we go": {
				matched:    false,
				event:      "and so we go",
				matchCount: 0,
			},
		}

		// this test is about being over-cautious, making sure the Scan method
		// will never cause a race when calling the Delete method at the same time.
		// It can't happen with the current implementation / concurrency pattern
		// used in processor.go, but I'm being over-cautious because if it happens
		// in the future because of someone changing the processor implementation,
		// it could lead to a panic and a hard crash of the Agent.

		go func() {
			for {
				for k := range tests {
					msg := message.Message{}
					s.Scan([]byte(k), &msg)
				}
			}
		}()

		time.Sleep(100 * time.Millisecond)
		s.Delete()
	}
}

func TestInterpretRC(t *testing.T) {
	require := require.New(t)

	defaults := StandardRulesDefaults{
		IncludedKeywordsCharCount: 10,
		ExcludedKeywordsCharCount: 10,
		ExcludedKeywords:          []string{"trace-id"},
	}

	stdRc := StandardRuleConfig{
		ID:          "0",
		Name:        "Zero",
		Description: "Zero desc",
		Definitions: []StandardRuleDefinition{{
			Version: 1,
			Pattern: "rule pattern 1",
		}},
	}

	rc := RuleConfig{
		Name:        "test",
		Description: "desc",
		Definition: RuleDefinition{
			StandardRuleID: "0",
		},
		Tags: []string{"tag:test"},
		MatchAction: MatchAction{
			Type:        matchActionRCRedact,
			Placeholder: "[redacted]",
		},
		IncludedKeywords: ProximityKeywords{
			UseRecommendedKeywords: true,
		},
	}

	rule, err := interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok := rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "rule pattern 1")
	require.Equal(rxRule.SecondaryValidator, sds.SecondaryValidator(""))

	// add a version with a required capability
	stdRc.Definitions = append(stdRc.Definitions, StandardRuleDefinition{
		Version:              2,
		Pattern:              "second pattern",
		RequiredCapabilities: []string{RCSecondaryValidationLuhnChecksum},
	})

	rule, err = interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok = rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "second pattern")
	require.Equal(rxRule.SecondaryValidator, sds.LuhnChecksum)

	// add a third version with an unknown required capability
	// it should fallback on using the version 2
	// also, make sure the version ain't ordered properly
	stdRc.Definitions = []StandardRuleDefinition{
		{
			Version:              2,
			Pattern:              "second pattern",
			RequiredCapabilities: []string{RCSecondaryValidationLuhnChecksum},
		},
		{
			Version:              1,
			Pattern:              "first pattern",
			RequiredCapabilities: nil,
		},
		{
			Version:              3,
			Pattern:              "third pattern",
			RequiredCapabilities: []string{"unsupported"},
		},
	}

	rule, err = interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok = rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "second pattern")
	require.Equal(rxRule.SecondaryValidator, sds.LuhnChecksum)

	// included keywords
	// -----------------

	// make sure we use the keywords proximity feature if any's configured
	// in the std rule definition
	stdRc.Definitions = []StandardRuleDefinition{
		{
			Version:                 2,
			Pattern:                 "second pattern",
			RequiredCapabilities:    []string{RCSecondaryValidationLuhnChecksum},
			DefaultIncludedKeywords: []string{"hello"},
		},
		{
			Version:              1,
			Pattern:              "first pattern",
			RequiredCapabilities: nil,
		},
	}

	rule, err = interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok = rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "second pattern")
	require.Equal(rxRule.SecondaryValidator, sds.LuhnChecksum)
	require.NotNil(rxRule.ProximityKeywords)
	require.Equal(rxRule.ProximityKeywords.LookAheadCharacterCount, uint32(10))
	require.Equal(rxRule.ProximityKeywords.IncludedKeywords, []string{"hello"})

	// make sure we use the user provided information first
	// even if there is some in the std rule
	rc.IncludedKeywords = ProximityKeywords{
		Keywords:               []string{"custom"},
		CharacterCount:         42,
		UseRecommendedKeywords: false,
	}

	rule, err = interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok = rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "second pattern")
	require.Equal(rxRule.SecondaryValidator, sds.LuhnChecksum)
	require.NotNil(rxRule.ProximityKeywords)
	require.Equal(rxRule.ProximityKeywords.LookAheadCharacterCount, uint32(42))
	require.Equal(rxRule.ProximityKeywords.IncludedKeywords, []string{"custom"})

	// excluded keywords
	// -----------------

	// make sure we use the user provided information first
	// even if there is some in the std rule
	rc.IncludedKeywords = ProximityKeywords{
		Keywords:               nil,
		CharacterCount:         0,
		UseRecommendedKeywords: false,
	}

	// make sure we use the keywords proximity feature if any's configured
	// in the std rule definition, here the excluded keywords one
	stdRc.Definitions = []StandardRuleDefinition{
		{
			Version:              2,
			Pattern:              "second pattern",
			RequiredCapabilities: []string{RCSecondaryValidationLuhnChecksum},
		},
		{
			Version:              1,
			Pattern:              "first pattern",
			RequiredCapabilities: nil,
		},
	}

	rule, err = interpretRCRule(rc, stdRc, defaults)
	require.NoError(err)
	rxRule, ok = rule.(sds.RegexRuleConfig)
	require.True(ok)

	require.Equal(rxRule.Id, "Zero")
	require.Equal(rxRule.Pattern, "second pattern")
	require.Equal(rxRule.SecondaryValidator, sds.LuhnChecksum)
	require.NotNil(rxRule.ProximityKeywords)
	require.Equal(rxRule.ProximityKeywords.LookAheadCharacterCount, uint32(10))
	require.Equal(rxRule.ProximityKeywords.ExcludedKeywords, []string{"trace-id"})
}
