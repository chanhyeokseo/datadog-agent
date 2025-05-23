// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package eval holds eval related files
package eval

import (
	"fmt"

	"github.com/DataDog/datadog-agent/pkg/security/secl/compiler/ast"
)

// MacroID - ID of a Macro
type MacroID = string

// Macro - Macro object identified by an `ID` containing a SECL `Expression`
type Macro struct {
	ID   MacroID
	Opts *Opts

	evaluator *MacroEvaluator
	ast       *ast.Macro
}

// MacroEvaluator - Evaluation part of a Macro
type MacroEvaluator struct {
	Value     interface{}
	EventType EventType

	fieldValues map[Field][]FieldValue
	fields      []Field
}

// NewMacro parses an expression and returns a new macro
func NewMacro(id, expression string, model Model, parsingContext *ast.ParsingContext, opts *Opts) (*Macro, error) {
	macro := &Macro{
		ID:   id,
		Opts: opts,
	}

	if err := macro.Parse(parsingContext, expression); err != nil {
		return nil, fmt.Errorf("syntax error: %w", err)
	}

	if err := macro.GenEvaluator(expression, model); err != nil {
		return nil, fmt.Errorf("compilation error: %w", err)
	}

	return macro, nil
}

// NewStringValuesMacro returns a new macro from an array of strings
func NewStringValuesMacro(id string, values []string, opts *Opts) (*Macro, error) {
	var evaluator StringValuesEvaluator
	for _, value := range values {
		fieldValue := FieldValue{
			Type:  ScalarValueType,
			Value: value,
		}

		evaluator.Values.AppendFieldValue(fieldValue)
	}

	if err := evaluator.Compile(DefaultStringCmpOpts); err != nil {
		return nil, err
	}

	return &Macro{
		ID:        id,
		Opts:      opts,
		evaluator: &MacroEvaluator{Value: &evaluator},
	}, nil
}

// GetEvaluator - Returns the MacroEvaluator of the Macro corresponding to the SECL `Expression`
func (m *Macro) GetEvaluator() *MacroEvaluator {
	return m.evaluator
}

// GetAst - Returns the representation of the SECL `Expression`
func (m *Macro) GetAst() *ast.Macro {
	return m.ast
}

// Parse - Transforms the SECL `Expression` into its AST representation
func (m *Macro) Parse(parsingContext *ast.ParsingContext, expression string) error {
	astMacro, err := parsingContext.ParseMacro(expression)
	if err != nil {
		return err
	}
	m.ast = astMacro
	return nil
}

func macroToEvaluator(macro *ast.Macro, model Model, opts *Opts, field Field) (*MacroEvaluator, error) {
	state := NewState(model, field, opts.MacroStore)

	var eval interface{}
	var err error

	switch {
	case macro.Expression != nil:
		eval, _, err = nodeToEvaluator(macro.Expression, opts, state)
	case macro.Array != nil:
		eval, _, err = nodeToEvaluator(macro.Array, opts, state)
	case macro.Primary != nil:
		eval, _, err = nodeToEvaluator(macro.Primary, opts, state)
	}

	if err != nil {
		return nil, err
	}

	eventType, err := eventTypeFromFields(model, state)
	if err != nil {
		return nil, err
	}

	return &MacroEvaluator{
		Value:     eval,
		EventType: eventType,

		fieldValues: state.fieldValues,
		fields:      KeysOfMap(state.fieldValues),
	}, nil
}

// GenEvaluator - Compiles and generates the evalutor
func (m *Macro) GenEvaluator(expression string, model Model) error {
	evaluator, err := macroToEvaluator(m.ast, model, m.Opts, "")
	if err != nil {
		if err, ok := err.(*ErrAstToEval); ok {
			return fmt.Errorf("macro syntax error: %w", &ErrRuleParse{pos: err.Pos, expr: expression})
		}
		return fmt.Errorf("macro compilation error: %w", err)
	}
	m.evaluator = evaluator

	return nil
}

// GetEventType - Returns the Event Type that the `Expression` handles
func (m *Macro) GetEventType() EventType {
	return m.evaluator.EventType
}

// GetFields - Returns all the Field that the Macro handles included sub-Macro
func (m *Macro) GetFields() []Field {
	return m.evaluator.GetFields()
}

// GetFields - Returns all the Field that the MacroEvaluator handles
func (m *MacroEvaluator) GetFields() []Field {
	return m.fields
}
