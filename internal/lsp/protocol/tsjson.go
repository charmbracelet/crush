// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated for LSP. DO NOT EDIT.

package protocol

// Code generated from protocol/metaModel.json at ref release/protocol/3.17.6-next.9 (hash c94395b5da53729e6dff931293b051009ccaaaa4).
// https://github.com/microsoft/vscode-languageserver-node/blob/release/protocol/3.17.6-next.9/protocol/metaModel.json
// LSP metaData.version = 3.17.0.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// UnmarshalError indicates that a JSON value did not conform to
// one of the expected cases of an LSP union type.
type UnmarshalError struct {
	msg string
}

func (e UnmarshalError) Error() string {
	return e.msg
}
func (t Or_CancelParams_id) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case int32:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [int32 string]", t)
}

func (t *Or_CancelParams_id) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder41 := json.NewDecoder(bytes.NewReader(x))
	decoder41.DisallowUnknownFields()
	var int32Val int32
	if err := decoder41.Decode(&int32Val); err == nil {
		t.Value = int32Val
		return nil
	}
	decoder42 := json.NewDecoder(bytes.NewReader(x))
	decoder42.DisallowUnknownFields()
	var stringVal string
	if err := decoder42.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [int32 string]"}
}

func (t Or_ClientSemanticTokensRequestOptions_full) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case ClientSemanticTokensRequestFullDelta:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [ClientSemanticTokensRequestFullDelta bool]", t)
}

func (t *Or_ClientSemanticTokensRequestOptions_full) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder220 := json.NewDecoder(bytes.NewReader(x))
	decoder220.DisallowUnknownFields()
	var boolVal bool
	if err := decoder220.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder221 := json.NewDecoder(bytes.NewReader(x))
	decoder221.DisallowUnknownFields()
	var h221 ClientSemanticTokensRequestFullDelta
	if err := decoder221.Decode(&h221); err == nil {
		t.Value = h221
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [ClientSemanticTokensRequestFullDelta bool]"}
}

func (t Or_ClientSemanticTokensRequestOptions_range) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Lit_ClientSemanticTokensRequestOptions_range_Item1:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Lit_ClientSemanticTokensRequestOptions_range_Item1 bool]", t)
}

func (t *Or_ClientSemanticTokensRequestOptions_range) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder217 := json.NewDecoder(bytes.NewReader(x))
	decoder217.DisallowUnknownFields()
	var boolVal bool
	if err := decoder217.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder218 := json.NewDecoder(bytes.NewReader(x))
	decoder218.DisallowUnknownFields()
	var h218 Lit_ClientSemanticTokensRequestOptions_range_Item1
	if err := decoder218.Decode(&h218); err == nil {
		t.Value = h218
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Lit_ClientSemanticTokensRequestOptions_range_Item1 bool]"}
}

func (t Or_CompletionItemDefaults_editRange) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case EditRangeWithInsertReplace:
		return json.Marshal(x)
	case Range:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [EditRangeWithInsertReplace Range]", t)
}

func (t *Or_CompletionItemDefaults_editRange) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder183 := json.NewDecoder(bytes.NewReader(x))
	decoder183.DisallowUnknownFields()
	var h183 EditRangeWithInsertReplace
	if err := decoder183.Decode(&h183); err == nil {
		t.Value = h183
		return nil
	}
	decoder184 := json.NewDecoder(bytes.NewReader(x))
	decoder184.DisallowUnknownFields()
	var h184 Range
	if err := decoder184.Decode(&h184); err == nil {
		t.Value = h184
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [EditRangeWithInsertReplace Range]"}
}

func (t Or_CompletionItem_documentation) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkupContent:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkupContent string]", t)
}

func (t *Or_CompletionItem_documentation) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder25 := json.NewDecoder(bytes.NewReader(x))
	decoder25.DisallowUnknownFields()
	var stringVal string
	if err := decoder25.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder26 := json.NewDecoder(bytes.NewReader(x))
	decoder26.DisallowUnknownFields()
	var h26 MarkupContent
	if err := decoder26.Decode(&h26); err == nil {
		t.Value = h26
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkupContent string]"}
}

func (t Or_CompletionItem_textEdit) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InsertReplaceEdit:
		return json.Marshal(x)
	case TextEdit:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InsertReplaceEdit TextEdit]", t)
}

func (t *Or_CompletionItem_textEdit) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder29 := json.NewDecoder(bytes.NewReader(x))
	decoder29.DisallowUnknownFields()
	var h29 InsertReplaceEdit
	if err := decoder29.Decode(&h29); err == nil {
		t.Value = h29
		return nil
	}
	decoder30 := json.NewDecoder(bytes.NewReader(x))
	decoder30.DisallowUnknownFields()
	var h30 TextEdit
	if err := decoder30.Decode(&h30); err == nil {
		t.Value = h30
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InsertReplaceEdit TextEdit]"}
}

func (t Or_Declaration) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Location:
		return json.Marshal(x)
	case []Location:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Location []Location]", t)
}

func (t *Or_Declaration) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder237 := json.NewDecoder(bytes.NewReader(x))
	decoder237.DisallowUnknownFields()
	var h237 Location
	if err := decoder237.Decode(&h237); err == nil {
		t.Value = h237
		return nil
	}
	decoder238 := json.NewDecoder(bytes.NewReader(x))
	decoder238.DisallowUnknownFields()
	var h238 []Location
	if err := decoder238.Decode(&h238); err == nil {
		t.Value = h238
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Location []Location]"}
}

func (t Or_Definition) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Location:
		return json.Marshal(x)
	case []Location:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Location []Location]", t)
}

func (t *Or_Definition) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder224 := json.NewDecoder(bytes.NewReader(x))
	decoder224.DisallowUnknownFields()
	var h224 Location
	if err := decoder224.Decode(&h224); err == nil {
		t.Value = h224
		return nil
	}
	decoder225 := json.NewDecoder(bytes.NewReader(x))
	decoder225.DisallowUnknownFields()
	var h225 []Location
	if err := decoder225.Decode(&h225); err == nil {
		t.Value = h225
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Location []Location]"}
}

func (t Or_Diagnostic_code) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case int32:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [int32 string]", t)
}

func (t *Or_Diagnostic_code) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder179 := json.NewDecoder(bytes.NewReader(x))
	decoder179.DisallowUnknownFields()
	var int32Val int32
	if err := decoder179.Decode(&int32Val); err == nil {
		t.Value = int32Val
		return nil
	}
	decoder180 := json.NewDecoder(bytes.NewReader(x))
	decoder180.DisallowUnknownFields()
	var stringVal string
	if err := decoder180.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [int32 string]"}
}

func (t Or_DidChangeConfigurationRegistrationOptions_section) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case []string:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [[]string string]", t)
}

func (t *Or_DidChangeConfigurationRegistrationOptions_section) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder22 := json.NewDecoder(bytes.NewReader(x))
	decoder22.DisallowUnknownFields()
	var stringVal string
	if err := decoder22.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder23 := json.NewDecoder(bytes.NewReader(x))
	decoder23.DisallowUnknownFields()
	var h23 []string
	if err := decoder23.Decode(&h23); err == nil {
		t.Value = h23
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [[]string string]"}
}

func (t Or_DocumentDiagnosticReport) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case RelatedFullDocumentDiagnosticReport:
		return json.Marshal(x)
	case RelatedUnchangedDocumentDiagnosticReport:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [RelatedFullDocumentDiagnosticReport RelatedUnchangedDocumentDiagnosticReport]", t)
}

func (t *Or_DocumentDiagnosticReport) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder247 := json.NewDecoder(bytes.NewReader(x))
	decoder247.DisallowUnknownFields()
	var h247 RelatedFullDocumentDiagnosticReport
	if err := decoder247.Decode(&h247); err == nil {
		t.Value = h247
		return nil
	}
	decoder248 := json.NewDecoder(bytes.NewReader(x))
	decoder248.DisallowUnknownFields()
	var h248 RelatedUnchangedDocumentDiagnosticReport
	if err := decoder248.Decode(&h248); err == nil {
		t.Value = h248
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [RelatedFullDocumentDiagnosticReport RelatedUnchangedDocumentDiagnosticReport]"}
}

func (t Or_DocumentDiagnosticReportPartialResult_relatedDocuments_Value) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case FullDocumentDiagnosticReport:
		return json.Marshal(x)
	case UnchangedDocumentDiagnosticReport:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]", t)
}

func (t *Or_DocumentDiagnosticReportPartialResult_relatedDocuments_Value) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder16 := json.NewDecoder(bytes.NewReader(x))
	decoder16.DisallowUnknownFields()
	var h16 FullDocumentDiagnosticReport
	if err := decoder16.Decode(&h16); err == nil {
		t.Value = h16
		return nil
	}
	decoder17 := json.NewDecoder(bytes.NewReader(x))
	decoder17.DisallowUnknownFields()
	var h17 UnchangedDocumentDiagnosticReport
	if err := decoder17.Decode(&h17); err == nil {
		t.Value = h17
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]"}
}

func (t Or_DocumentFilter) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookCellTextDocumentFilter:
		return json.Marshal(x)
	case TextDocumentFilter:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookCellTextDocumentFilter TextDocumentFilter]", t)
}

func (t *Or_DocumentFilter) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder270 := json.NewDecoder(bytes.NewReader(x))
	decoder270.DisallowUnknownFields()
	var h270 NotebookCellTextDocumentFilter
	if err := decoder270.Decode(&h270); err == nil {
		t.Value = h270
		return nil
	}
	decoder271 := json.NewDecoder(bytes.NewReader(x))
	decoder271.DisallowUnknownFields()
	var h271 TextDocumentFilter
	if err := decoder271.Decode(&h271); err == nil {
		t.Value = h271
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookCellTextDocumentFilter TextDocumentFilter]"}
}

func (t Or_GlobPattern) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Pattern:
		return json.Marshal(x)
	case RelativePattern:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Pattern RelativePattern]", t)
}

func (t *Or_GlobPattern) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder274 := json.NewDecoder(bytes.NewReader(x))
	decoder274.DisallowUnknownFields()
	var h274 Pattern
	if err := decoder274.Decode(&h274); err == nil {
		t.Value = h274
		return nil
	}
	decoder275 := json.NewDecoder(bytes.NewReader(x))
	decoder275.DisallowUnknownFields()
	var h275 RelativePattern
	if err := decoder275.Decode(&h275); err == nil {
		t.Value = h275
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Pattern RelativePattern]"}
}

func (t Or_Hover_contents) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkedString:
		return json.Marshal(x)
	case MarkupContent:
		return json.Marshal(x)
	case []MarkedString:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkedString MarkupContent []MarkedString]", t)
}

func (t *Or_Hover_contents) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder34 := json.NewDecoder(bytes.NewReader(x))
	decoder34.DisallowUnknownFields()
	var h34 MarkedString
	if err := decoder34.Decode(&h34); err == nil {
		t.Value = h34
		return nil
	}
	decoder35 := json.NewDecoder(bytes.NewReader(x))
	decoder35.DisallowUnknownFields()
	var h35 MarkupContent
	if err := decoder35.Decode(&h35); err == nil {
		t.Value = h35
		return nil
	}
	decoder36 := json.NewDecoder(bytes.NewReader(x))
	decoder36.DisallowUnknownFields()
	var h36 []MarkedString
	if err := decoder36.Decode(&h36); err == nil {
		t.Value = h36
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkedString MarkupContent []MarkedString]"}
}

func (t Or_InlayHintLabelPart_tooltip) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkupContent:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkupContent string]", t)
}

func (t *Or_InlayHintLabelPart_tooltip) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder56 := json.NewDecoder(bytes.NewReader(x))
	decoder56.DisallowUnknownFields()
	var stringVal string
	if err := decoder56.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder57 := json.NewDecoder(bytes.NewReader(x))
	decoder57.DisallowUnknownFields()
	var h57 MarkupContent
	if err := decoder57.Decode(&h57); err == nil {
		t.Value = h57
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkupContent string]"}
}

func (t Or_InlayHint_label) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case []InlayHintLabelPart:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [[]InlayHintLabelPart string]", t)
}

func (t *Or_InlayHint_label) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder9 := json.NewDecoder(bytes.NewReader(x))
	decoder9.DisallowUnknownFields()
	var stringVal string
	if err := decoder9.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder10 := json.NewDecoder(bytes.NewReader(x))
	decoder10.DisallowUnknownFields()
	var h10 []InlayHintLabelPart
	if err := decoder10.Decode(&h10); err == nil {
		t.Value = h10
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [[]InlayHintLabelPart string]"}
}

func (t Or_InlayHint_tooltip) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkupContent:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkupContent string]", t)
}

func (t *Or_InlayHint_tooltip) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder12 := json.NewDecoder(bytes.NewReader(x))
	decoder12.DisallowUnknownFields()
	var stringVal string
	if err := decoder12.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder13 := json.NewDecoder(bytes.NewReader(x))
	decoder13.DisallowUnknownFields()
	var h13 MarkupContent
	if err := decoder13.Decode(&h13); err == nil {
		t.Value = h13
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkupContent string]"}
}

func (t Or_InlineCompletionItem_insertText) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case StringValue:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [StringValue string]", t)
}

func (t *Or_InlineCompletionItem_insertText) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder19 := json.NewDecoder(bytes.NewReader(x))
	decoder19.DisallowUnknownFields()
	var stringVal string
	if err := decoder19.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder20 := json.NewDecoder(bytes.NewReader(x))
	decoder20.DisallowUnknownFields()
	var h20 StringValue
	if err := decoder20.Decode(&h20); err == nil {
		t.Value = h20
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [StringValue string]"}
}

func (t Or_InlineValue) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InlineValueEvaluatableExpression:
		return json.Marshal(x)
	case InlineValueText:
		return json.Marshal(x)
	case InlineValueVariableLookup:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InlineValueEvaluatableExpression InlineValueText InlineValueVariableLookup]", t)
}

func (t *Or_InlineValue) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder242 := json.NewDecoder(bytes.NewReader(x))
	decoder242.DisallowUnknownFields()
	var h242 InlineValueEvaluatableExpression
	if err := decoder242.Decode(&h242); err == nil {
		t.Value = h242
		return nil
	}
	decoder243 := json.NewDecoder(bytes.NewReader(x))
	decoder243.DisallowUnknownFields()
	var h243 InlineValueText
	if err := decoder243.Decode(&h243); err == nil {
		t.Value = h243
		return nil
	}
	decoder244 := json.NewDecoder(bytes.NewReader(x))
	decoder244.DisallowUnknownFields()
	var h244 InlineValueVariableLookup
	if err := decoder244.Decode(&h244); err == nil {
		t.Value = h244
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InlineValueEvaluatableExpression InlineValueText InlineValueVariableLookup]"}
}

func (t Or_LSPAny) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case LSPArray:
		return json.Marshal(x)
	case LSPObject:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case float64:
		return json.Marshal(x)
	case int32:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case uint32:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [LSPArray LSPObject bool float64 int32 string uint32]", t)
}

func (t *Or_LSPAny) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder228 := json.NewDecoder(bytes.NewReader(x))
	decoder228.DisallowUnknownFields()
	var boolVal bool
	if err := decoder228.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder229 := json.NewDecoder(bytes.NewReader(x))
	decoder229.DisallowUnknownFields()
	var float64Val float64
	if err := decoder229.Decode(&float64Val); err == nil {
		t.Value = float64Val
		return nil
	}
	decoder230 := json.NewDecoder(bytes.NewReader(x))
	decoder230.DisallowUnknownFields()
	var int32Val int32
	if err := decoder230.Decode(&int32Val); err == nil {
		t.Value = int32Val
		return nil
	}
	decoder231 := json.NewDecoder(bytes.NewReader(x))
	decoder231.DisallowUnknownFields()
	var stringVal string
	if err := decoder231.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder232 := json.NewDecoder(bytes.NewReader(x))
	decoder232.DisallowUnknownFields()
	var uint32Val uint32
	if err := decoder232.Decode(&uint32Val); err == nil {
		t.Value = uint32Val
		return nil
	}
	decoder233 := json.NewDecoder(bytes.NewReader(x))
	decoder233.DisallowUnknownFields()
	var h233 LSPArray
	if err := decoder233.Decode(&h233); err == nil {
		t.Value = h233
		return nil
	}
	decoder234 := json.NewDecoder(bytes.NewReader(x))
	decoder234.DisallowUnknownFields()
	var h234 LSPObject
	if err := decoder234.Decode(&h234); err == nil {
		t.Value = h234
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [LSPArray LSPObject bool float64 int32 string uint32]"}
}

func (t Or_MarkedString) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkedStringWithLanguage:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkedStringWithLanguage string]", t)
}

func (t *Or_MarkedString) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder266 := json.NewDecoder(bytes.NewReader(x))
	decoder266.DisallowUnknownFields()
	var stringVal string
	if err := decoder266.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder267 := json.NewDecoder(bytes.NewReader(x))
	decoder267.DisallowUnknownFields()
	var h267 MarkedStringWithLanguage
	if err := decoder267.Decode(&h267); err == nil {
		t.Value = h267
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkedStringWithLanguage string]"}
}

func (t Or_NotebookCellTextDocumentFilter_notebook) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentFilter:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentFilter string]", t)
}

func (t *Or_NotebookCellTextDocumentFilter_notebook) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder208 := json.NewDecoder(bytes.NewReader(x))
	decoder208.DisallowUnknownFields()
	var stringVal string
	if err := decoder208.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder209 := json.NewDecoder(bytes.NewReader(x))
	decoder209.DisallowUnknownFields()
	var h209 NotebookDocumentFilter
	if err := decoder209.Decode(&h209); err == nil {
		t.Value = h209
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentFilter string]"}
}

func (t Or_NotebookDocumentFilter) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentFilterNotebookType:
		return json.Marshal(x)
	case NotebookDocumentFilterPattern:
		return json.Marshal(x)
	case NotebookDocumentFilterScheme:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentFilterNotebookType NotebookDocumentFilterPattern NotebookDocumentFilterScheme]", t)
}

func (t *Or_NotebookDocumentFilter) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder285 := json.NewDecoder(bytes.NewReader(x))
	decoder285.DisallowUnknownFields()
	var h285 NotebookDocumentFilterNotebookType
	if err := decoder285.Decode(&h285); err == nil {
		t.Value = h285
		return nil
	}
	decoder286 := json.NewDecoder(bytes.NewReader(x))
	decoder286.DisallowUnknownFields()
	var h286 NotebookDocumentFilterPattern
	if err := decoder286.Decode(&h286); err == nil {
		t.Value = h286
		return nil
	}
	decoder287 := json.NewDecoder(bytes.NewReader(x))
	decoder287.DisallowUnknownFields()
	var h287 NotebookDocumentFilterScheme
	if err := decoder287.Decode(&h287); err == nil {
		t.Value = h287
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentFilterNotebookType NotebookDocumentFilterPattern NotebookDocumentFilterScheme]"}
}

func (t Or_NotebookDocumentFilterWithCells_notebook) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentFilter:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentFilter string]", t)
}

func (t *Or_NotebookDocumentFilterWithCells_notebook) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder192 := json.NewDecoder(bytes.NewReader(x))
	decoder192.DisallowUnknownFields()
	var stringVal string
	if err := decoder192.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder193 := json.NewDecoder(bytes.NewReader(x))
	decoder193.DisallowUnknownFields()
	var h193 NotebookDocumentFilter
	if err := decoder193.Decode(&h193); err == nil {
		t.Value = h193
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentFilter string]"}
}

func (t Or_NotebookDocumentFilterWithNotebook_notebook) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentFilter:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentFilter string]", t)
}

func (t *Or_NotebookDocumentFilterWithNotebook_notebook) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder189 := json.NewDecoder(bytes.NewReader(x))
	decoder189.DisallowUnknownFields()
	var stringVal string
	if err := decoder189.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder190 := json.NewDecoder(bytes.NewReader(x))
	decoder190.DisallowUnknownFields()
	var h190 NotebookDocumentFilter
	if err := decoder190.Decode(&h190); err == nil {
		t.Value = h190
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentFilter string]"}
}

func (t Or_NotebookDocumentSyncOptions_notebookSelector_Elem) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentFilterWithCells:
		return json.Marshal(x)
	case NotebookDocumentFilterWithNotebook:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentFilterWithCells NotebookDocumentFilterWithNotebook]", t)
}

func (t *Or_NotebookDocumentSyncOptions_notebookSelector_Elem) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder68 := json.NewDecoder(bytes.NewReader(x))
	decoder68.DisallowUnknownFields()
	var h68 NotebookDocumentFilterWithCells
	if err := decoder68.Decode(&h68); err == nil {
		t.Value = h68
		return nil
	}
	decoder69 := json.NewDecoder(bytes.NewReader(x))
	decoder69.DisallowUnknownFields()
	var h69 NotebookDocumentFilterWithNotebook
	if err := decoder69.Decode(&h69); err == nil {
		t.Value = h69
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentFilterWithCells NotebookDocumentFilterWithNotebook]"}
}

func (t Or_ParameterInformation_documentation) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkupContent:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkupContent string]", t)
}

func (t *Or_ParameterInformation_documentation) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder205 := json.NewDecoder(bytes.NewReader(x))
	decoder205.DisallowUnknownFields()
	var stringVal string
	if err := decoder205.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder206 := json.NewDecoder(bytes.NewReader(x))
	decoder206.DisallowUnknownFields()
	var h206 MarkupContent
	if err := decoder206.Decode(&h206); err == nil {
		t.Value = h206
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkupContent string]"}
}

func (t Or_ParameterInformation_label) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Tuple_ParameterInformation_label_Item1:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Tuple_ParameterInformation_label_Item1 string]", t)
}

func (t *Or_ParameterInformation_label) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder202 := json.NewDecoder(bytes.NewReader(x))
	decoder202.DisallowUnknownFields()
	var stringVal string
	if err := decoder202.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder203 := json.NewDecoder(bytes.NewReader(x))
	decoder203.DisallowUnknownFields()
	var h203 Tuple_ParameterInformation_label_Item1
	if err := decoder203.Decode(&h203); err == nil {
		t.Value = h203
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Tuple_ParameterInformation_label_Item1 string]"}
}

func (t Or_PrepareRenameResult) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case PrepareRenameDefaultBehavior:
		return json.Marshal(x)
	case PrepareRenamePlaceholder:
		return json.Marshal(x)
	case Range:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [PrepareRenameDefaultBehavior PrepareRenamePlaceholder Range]", t)
}

func (t *Or_PrepareRenameResult) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder252 := json.NewDecoder(bytes.NewReader(x))
	decoder252.DisallowUnknownFields()
	var h252 PrepareRenameDefaultBehavior
	if err := decoder252.Decode(&h252); err == nil {
		t.Value = h252
		return nil
	}
	decoder253 := json.NewDecoder(bytes.NewReader(x))
	decoder253.DisallowUnknownFields()
	var h253 PrepareRenamePlaceholder
	if err := decoder253.Decode(&h253); err == nil {
		t.Value = h253
		return nil
	}
	decoder254 := json.NewDecoder(bytes.NewReader(x))
	decoder254.DisallowUnknownFields()
	var h254 Range
	if err := decoder254.Decode(&h254); err == nil {
		t.Value = h254
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [PrepareRenameDefaultBehavior PrepareRenamePlaceholder Range]"}
}

func (t Or_ProgressToken) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case int32:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [int32 string]", t)
}

func (t *Or_ProgressToken) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder255 := json.NewDecoder(bytes.NewReader(x))
	decoder255.DisallowUnknownFields()
	var int32Val int32
	if err := decoder255.Decode(&int32Val); err == nil {
		t.Value = int32Val
		return nil
	}
	decoder256 := json.NewDecoder(bytes.NewReader(x))
	decoder256.DisallowUnknownFields()
	var stringVal string
	if err := decoder256.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [int32 string]"}
}

func (t Or_RelatedFullDocumentDiagnosticReport_relatedDocuments_Value) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case FullDocumentDiagnosticReport:
		return json.Marshal(x)
	case UnchangedDocumentDiagnosticReport:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]", t)
}

func (t *Or_RelatedFullDocumentDiagnosticReport_relatedDocuments_Value) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder60 := json.NewDecoder(bytes.NewReader(x))
	decoder60.DisallowUnknownFields()
	var h60 FullDocumentDiagnosticReport
	if err := decoder60.Decode(&h60); err == nil {
		t.Value = h60
		return nil
	}
	decoder61 := json.NewDecoder(bytes.NewReader(x))
	decoder61.DisallowUnknownFields()
	var h61 UnchangedDocumentDiagnosticReport
	if err := decoder61.Decode(&h61); err == nil {
		t.Value = h61
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]"}
}

func (t Or_RelatedUnchangedDocumentDiagnosticReport_relatedDocuments_Value) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case FullDocumentDiagnosticReport:
		return json.Marshal(x)
	case UnchangedDocumentDiagnosticReport:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]", t)
}

func (t *Or_RelatedUnchangedDocumentDiagnosticReport_relatedDocuments_Value) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder64 := json.NewDecoder(bytes.NewReader(x))
	decoder64.DisallowUnknownFields()
	var h64 FullDocumentDiagnosticReport
	if err := decoder64.Decode(&h64); err == nil {
		t.Value = h64
		return nil
	}
	decoder65 := json.NewDecoder(bytes.NewReader(x))
	decoder65.DisallowUnknownFields()
	var h65 UnchangedDocumentDiagnosticReport
	if err := decoder65.Decode(&h65); err == nil {
		t.Value = h65
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [FullDocumentDiagnosticReport UnchangedDocumentDiagnosticReport]"}
}

func (t Or_RelativePattern_baseUri) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case URI:
		return json.Marshal(x)
	case WorkspaceFolder:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [URI WorkspaceFolder]", t)
}

func (t *Or_RelativePattern_baseUri) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder214 := json.NewDecoder(bytes.NewReader(x))
	decoder214.DisallowUnknownFields()
	var h214 URI
	if err := decoder214.Decode(&h214); err == nil {
		t.Value = h214
		return nil
	}
	decoder215 := json.NewDecoder(bytes.NewReader(x))
	decoder215.DisallowUnknownFields()
	var h215 WorkspaceFolder
	if err := decoder215.Decode(&h215); err == nil {
		t.Value = h215
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [URI WorkspaceFolder]"}
}

func (t Or_Result_textDocument_codeAction_Item0_Elem) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case CodeAction:
		return json.Marshal(x)
	case Command:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [CodeAction Command]", t)
}

func (t *Or_Result_textDocument_codeAction_Item0_Elem) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder322 := json.NewDecoder(bytes.NewReader(x))
	decoder322.DisallowUnknownFields()
	var h322 CodeAction
	if err := decoder322.Decode(&h322); err == nil {
		t.Value = h322
		return nil
	}
	decoder323 := json.NewDecoder(bytes.NewReader(x))
	decoder323.DisallowUnknownFields()
	var h323 Command
	if err := decoder323.Decode(&h323); err == nil {
		t.Value = h323
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [CodeAction Command]"}
}

func (t Or_Result_textDocument_completion) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case CompletionList:
		return json.Marshal(x)
	case []CompletionItem:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [CompletionList []CompletionItem]", t)
}

func (t *Or_Result_textDocument_completion) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder310 := json.NewDecoder(bytes.NewReader(x))
	decoder310.DisallowUnknownFields()
	var h310 CompletionList
	if err := decoder310.Decode(&h310); err == nil {
		t.Value = h310
		return nil
	}
	decoder311 := json.NewDecoder(bytes.NewReader(x))
	decoder311.DisallowUnknownFields()
	var h311 []CompletionItem
	if err := decoder311.Decode(&h311); err == nil {
		t.Value = h311
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [CompletionList []CompletionItem]"}
}

func (t Or_Result_textDocument_declaration) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Declaration:
		return json.Marshal(x)
	case []DeclarationLink:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Declaration []DeclarationLink]", t)
}

func (t *Or_Result_textDocument_declaration) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder298 := json.NewDecoder(bytes.NewReader(x))
	decoder298.DisallowUnknownFields()
	var h298 Declaration
	if err := decoder298.Decode(&h298); err == nil {
		t.Value = h298
		return nil
	}
	decoder299 := json.NewDecoder(bytes.NewReader(x))
	decoder299.DisallowUnknownFields()
	var h299 []DeclarationLink
	if err := decoder299.Decode(&h299); err == nil {
		t.Value = h299
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Declaration []DeclarationLink]"}
}

func (t Or_Result_textDocument_definition) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Definition:
		return json.Marshal(x)
	case []DefinitionLink:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Definition []DefinitionLink]", t)
}

func (t *Or_Result_textDocument_definition) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder314 := json.NewDecoder(bytes.NewReader(x))
	decoder314.DisallowUnknownFields()
	var h314 Definition
	if err := decoder314.Decode(&h314); err == nil {
		t.Value = h314
		return nil
	}
	decoder315 := json.NewDecoder(bytes.NewReader(x))
	decoder315.DisallowUnknownFields()
	var h315 []DefinitionLink
	if err := decoder315.Decode(&h315); err == nil {
		t.Value = h315
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Definition []DefinitionLink]"}
}

func (t Or_Result_textDocument_documentSymbol) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case []DocumentSymbol:
		return json.Marshal(x)
	case []SymbolInformation:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [[]DocumentSymbol []SymbolInformation]", t)
}

func (t *Or_Result_textDocument_documentSymbol) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder318 := json.NewDecoder(bytes.NewReader(x))
	decoder318.DisallowUnknownFields()
	var h318 []DocumentSymbol
	if err := decoder318.Decode(&h318); err == nil {
		t.Value = h318
		return nil
	}
	decoder319 := json.NewDecoder(bytes.NewReader(x))
	decoder319.DisallowUnknownFields()
	var h319 []SymbolInformation
	if err := decoder319.Decode(&h319); err == nil {
		t.Value = h319
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [[]DocumentSymbol []SymbolInformation]"}
}

func (t Or_Result_textDocument_implementation) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Definition:
		return json.Marshal(x)
	case []DefinitionLink:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Definition []DefinitionLink]", t)
}

func (t *Or_Result_textDocument_implementation) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder290 := json.NewDecoder(bytes.NewReader(x))
	decoder290.DisallowUnknownFields()
	var h290 Definition
	if err := decoder290.Decode(&h290); err == nil {
		t.Value = h290
		return nil
	}
	decoder291 := json.NewDecoder(bytes.NewReader(x))
	decoder291.DisallowUnknownFields()
	var h291 []DefinitionLink
	if err := decoder291.Decode(&h291); err == nil {
		t.Value = h291
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Definition []DefinitionLink]"}
}

func (t Or_Result_textDocument_inlineCompletion) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InlineCompletionList:
		return json.Marshal(x)
	case []InlineCompletionItem:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InlineCompletionList []InlineCompletionItem]", t)
}

func (t *Or_Result_textDocument_inlineCompletion) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder306 := json.NewDecoder(bytes.NewReader(x))
	decoder306.DisallowUnknownFields()
	var h306 InlineCompletionList
	if err := decoder306.Decode(&h306); err == nil {
		t.Value = h306
		return nil
	}
	decoder307 := json.NewDecoder(bytes.NewReader(x))
	decoder307.DisallowUnknownFields()
	var h307 []InlineCompletionItem
	if err := decoder307.Decode(&h307); err == nil {
		t.Value = h307
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InlineCompletionList []InlineCompletionItem]"}
}

func (t Or_Result_textDocument_semanticTokens_full_delta) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case SemanticTokens:
		return json.Marshal(x)
	case SemanticTokensDelta:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [SemanticTokens SemanticTokensDelta]", t)
}

func (t *Or_Result_textDocument_semanticTokens_full_delta) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder302 := json.NewDecoder(bytes.NewReader(x))
	decoder302.DisallowUnknownFields()
	var h302 SemanticTokens
	if err := decoder302.Decode(&h302); err == nil {
		t.Value = h302
		return nil
	}
	decoder303 := json.NewDecoder(bytes.NewReader(x))
	decoder303.DisallowUnknownFields()
	var h303 SemanticTokensDelta
	if err := decoder303.Decode(&h303); err == nil {
		t.Value = h303
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [SemanticTokens SemanticTokensDelta]"}
}

func (t Or_Result_textDocument_typeDefinition) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Definition:
		return json.Marshal(x)
	case []DefinitionLink:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Definition []DefinitionLink]", t)
}

func (t *Or_Result_textDocument_typeDefinition) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder294 := json.NewDecoder(bytes.NewReader(x))
	decoder294.DisallowUnknownFields()
	var h294 Definition
	if err := decoder294.Decode(&h294); err == nil {
		t.Value = h294
		return nil
	}
	decoder295 := json.NewDecoder(bytes.NewReader(x))
	decoder295.DisallowUnknownFields()
	var h295 []DefinitionLink
	if err := decoder295.Decode(&h295); err == nil {
		t.Value = h295
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Definition []DefinitionLink]"}
}

func (t Or_Result_workspace_symbol) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case []SymbolInformation:
		return json.Marshal(x)
	case []WorkspaceSymbol:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [[]SymbolInformation []WorkspaceSymbol]", t)
}

func (t *Or_Result_workspace_symbol) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder326 := json.NewDecoder(bytes.NewReader(x))
	decoder326.DisallowUnknownFields()
	var h326 []SymbolInformation
	if err := decoder326.Decode(&h326); err == nil {
		t.Value = h326
		return nil
	}
	decoder327 := json.NewDecoder(bytes.NewReader(x))
	decoder327.DisallowUnknownFields()
	var h327 []WorkspaceSymbol
	if err := decoder327.Decode(&h327); err == nil {
		t.Value = h327
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [[]SymbolInformation []WorkspaceSymbol]"}
}

func (t Or_SemanticTokensOptions_full) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case SemanticTokensFullDelta:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [SemanticTokensFullDelta bool]", t)
}

func (t *Or_SemanticTokensOptions_full) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder47 := json.NewDecoder(bytes.NewReader(x))
	decoder47.DisallowUnknownFields()
	var boolVal bool
	if err := decoder47.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder48 := json.NewDecoder(bytes.NewReader(x))
	decoder48.DisallowUnknownFields()
	var h48 SemanticTokensFullDelta
	if err := decoder48.Decode(&h48); err == nil {
		t.Value = h48
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [SemanticTokensFullDelta bool]"}
}

func (t Or_SemanticTokensOptions_range) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Lit_SemanticTokensOptions_range_Item1:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Lit_SemanticTokensOptions_range_Item1 bool]", t)
}

func (t *Or_SemanticTokensOptions_range) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder44 := json.NewDecoder(bytes.NewReader(x))
	decoder44.DisallowUnknownFields()
	var boolVal bool
	if err := decoder44.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder45 := json.NewDecoder(bytes.NewReader(x))
	decoder45.DisallowUnknownFields()
	var h45 Lit_SemanticTokensOptions_range_Item1
	if err := decoder45.Decode(&h45); err == nil {
		t.Value = h45
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Lit_SemanticTokensOptions_range_Item1 bool]"}
}

func (t Or_ServerCapabilities_callHierarchyProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case CallHierarchyOptions:
		return json.Marshal(x)
	case CallHierarchyRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [CallHierarchyOptions CallHierarchyRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_callHierarchyProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder140 := json.NewDecoder(bytes.NewReader(x))
	decoder140.DisallowUnknownFields()
	var boolVal bool
	if err := decoder140.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder141 := json.NewDecoder(bytes.NewReader(x))
	decoder141.DisallowUnknownFields()
	var h141 CallHierarchyOptions
	if err := decoder141.Decode(&h141); err == nil {
		t.Value = h141
		return nil
	}
	decoder142 := json.NewDecoder(bytes.NewReader(x))
	decoder142.DisallowUnknownFields()
	var h142 CallHierarchyRegistrationOptions
	if err := decoder142.Decode(&h142); err == nil {
		t.Value = h142
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [CallHierarchyOptions CallHierarchyRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_codeActionProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case CodeActionOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [CodeActionOptions bool]", t)
}

func (t *Or_ServerCapabilities_codeActionProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder109 := json.NewDecoder(bytes.NewReader(x))
	decoder109.DisallowUnknownFields()
	var boolVal bool
	if err := decoder109.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder110 := json.NewDecoder(bytes.NewReader(x))
	decoder110.DisallowUnknownFields()
	var h110 CodeActionOptions
	if err := decoder110.Decode(&h110); err == nil {
		t.Value = h110
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [CodeActionOptions bool]"}
}

func (t Or_ServerCapabilities_colorProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DocumentColorOptions:
		return json.Marshal(x)
	case DocumentColorRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DocumentColorOptions DocumentColorRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_colorProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder113 := json.NewDecoder(bytes.NewReader(x))
	decoder113.DisallowUnknownFields()
	var boolVal bool
	if err := decoder113.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder114 := json.NewDecoder(bytes.NewReader(x))
	decoder114.DisallowUnknownFields()
	var h114 DocumentColorOptions
	if err := decoder114.Decode(&h114); err == nil {
		t.Value = h114
		return nil
	}
	decoder115 := json.NewDecoder(bytes.NewReader(x))
	decoder115.DisallowUnknownFields()
	var h115 DocumentColorRegistrationOptions
	if err := decoder115.Decode(&h115); err == nil {
		t.Value = h115
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DocumentColorOptions DocumentColorRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_declarationProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DeclarationOptions:
		return json.Marshal(x)
	case DeclarationRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DeclarationOptions DeclarationRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_declarationProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder83 := json.NewDecoder(bytes.NewReader(x))
	decoder83.DisallowUnknownFields()
	var boolVal bool
	if err := decoder83.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder84 := json.NewDecoder(bytes.NewReader(x))
	decoder84.DisallowUnknownFields()
	var h84 DeclarationOptions
	if err := decoder84.Decode(&h84); err == nil {
		t.Value = h84
		return nil
	}
	decoder85 := json.NewDecoder(bytes.NewReader(x))
	decoder85.DisallowUnknownFields()
	var h85 DeclarationRegistrationOptions
	if err := decoder85.Decode(&h85); err == nil {
		t.Value = h85
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DeclarationOptions DeclarationRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_definitionProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DefinitionOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DefinitionOptions bool]", t)
}

func (t *Or_ServerCapabilities_definitionProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder87 := json.NewDecoder(bytes.NewReader(x))
	decoder87.DisallowUnknownFields()
	var boolVal bool
	if err := decoder87.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder88 := json.NewDecoder(bytes.NewReader(x))
	decoder88.DisallowUnknownFields()
	var h88 DefinitionOptions
	if err := decoder88.Decode(&h88); err == nil {
		t.Value = h88
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DefinitionOptions bool]"}
}

func (t Or_ServerCapabilities_diagnosticProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DiagnosticOptions:
		return json.Marshal(x)
	case DiagnosticRegistrationOptions:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DiagnosticOptions DiagnosticRegistrationOptions]", t)
}

func (t *Or_ServerCapabilities_diagnosticProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder174 := json.NewDecoder(bytes.NewReader(x))
	decoder174.DisallowUnknownFields()
	var h174 DiagnosticOptions
	if err := decoder174.Decode(&h174); err == nil {
		t.Value = h174
		return nil
	}
	decoder175 := json.NewDecoder(bytes.NewReader(x))
	decoder175.DisallowUnknownFields()
	var h175 DiagnosticRegistrationOptions
	if err := decoder175.Decode(&h175); err == nil {
		t.Value = h175
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DiagnosticOptions DiagnosticRegistrationOptions]"}
}

func (t Or_ServerCapabilities_documentFormattingProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DocumentFormattingOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DocumentFormattingOptions bool]", t)
}

func (t *Or_ServerCapabilities_documentFormattingProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder120 := json.NewDecoder(bytes.NewReader(x))
	decoder120.DisallowUnknownFields()
	var boolVal bool
	if err := decoder120.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder121 := json.NewDecoder(bytes.NewReader(x))
	decoder121.DisallowUnknownFields()
	var h121 DocumentFormattingOptions
	if err := decoder121.Decode(&h121); err == nil {
		t.Value = h121
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DocumentFormattingOptions bool]"}
}

func (t Or_ServerCapabilities_documentHighlightProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DocumentHighlightOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DocumentHighlightOptions bool]", t)
}

func (t *Or_ServerCapabilities_documentHighlightProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder103 := json.NewDecoder(bytes.NewReader(x))
	decoder103.DisallowUnknownFields()
	var boolVal bool
	if err := decoder103.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder104 := json.NewDecoder(bytes.NewReader(x))
	decoder104.DisallowUnknownFields()
	var h104 DocumentHighlightOptions
	if err := decoder104.Decode(&h104); err == nil {
		t.Value = h104
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DocumentHighlightOptions bool]"}
}

func (t Or_ServerCapabilities_documentRangeFormattingProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DocumentRangeFormattingOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DocumentRangeFormattingOptions bool]", t)
}

func (t *Or_ServerCapabilities_documentRangeFormattingProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder123 := json.NewDecoder(bytes.NewReader(x))
	decoder123.DisallowUnknownFields()
	var boolVal bool
	if err := decoder123.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder124 := json.NewDecoder(bytes.NewReader(x))
	decoder124.DisallowUnknownFields()
	var h124 DocumentRangeFormattingOptions
	if err := decoder124.Decode(&h124); err == nil {
		t.Value = h124
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DocumentRangeFormattingOptions bool]"}
}

func (t Or_ServerCapabilities_documentSymbolProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case DocumentSymbolOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [DocumentSymbolOptions bool]", t)
}

func (t *Or_ServerCapabilities_documentSymbolProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder106 := json.NewDecoder(bytes.NewReader(x))
	decoder106.DisallowUnknownFields()
	var boolVal bool
	if err := decoder106.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder107 := json.NewDecoder(bytes.NewReader(x))
	decoder107.DisallowUnknownFields()
	var h107 DocumentSymbolOptions
	if err := decoder107.Decode(&h107); err == nil {
		t.Value = h107
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [DocumentSymbolOptions bool]"}
}

func (t Or_ServerCapabilities_foldingRangeProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case FoldingRangeOptions:
		return json.Marshal(x)
	case FoldingRangeRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [FoldingRangeOptions FoldingRangeRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_foldingRangeProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder130 := json.NewDecoder(bytes.NewReader(x))
	decoder130.DisallowUnknownFields()
	var boolVal bool
	if err := decoder130.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder131 := json.NewDecoder(bytes.NewReader(x))
	decoder131.DisallowUnknownFields()
	var h131 FoldingRangeOptions
	if err := decoder131.Decode(&h131); err == nil {
		t.Value = h131
		return nil
	}
	decoder132 := json.NewDecoder(bytes.NewReader(x))
	decoder132.DisallowUnknownFields()
	var h132 FoldingRangeRegistrationOptions
	if err := decoder132.Decode(&h132); err == nil {
		t.Value = h132
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [FoldingRangeOptions FoldingRangeRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_hoverProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case HoverOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [HoverOptions bool]", t)
}

func (t *Or_ServerCapabilities_hoverProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder79 := json.NewDecoder(bytes.NewReader(x))
	decoder79.DisallowUnknownFields()
	var boolVal bool
	if err := decoder79.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder80 := json.NewDecoder(bytes.NewReader(x))
	decoder80.DisallowUnknownFields()
	var h80 HoverOptions
	if err := decoder80.Decode(&h80); err == nil {
		t.Value = h80
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [HoverOptions bool]"}
}

func (t Or_ServerCapabilities_implementationProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case ImplementationOptions:
		return json.Marshal(x)
	case ImplementationRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [ImplementationOptions ImplementationRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_implementationProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder96 := json.NewDecoder(bytes.NewReader(x))
	decoder96.DisallowUnknownFields()
	var boolVal bool
	if err := decoder96.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder97 := json.NewDecoder(bytes.NewReader(x))
	decoder97.DisallowUnknownFields()
	var h97 ImplementationOptions
	if err := decoder97.Decode(&h97); err == nil {
		t.Value = h97
		return nil
	}
	decoder98 := json.NewDecoder(bytes.NewReader(x))
	decoder98.DisallowUnknownFields()
	var h98 ImplementationRegistrationOptions
	if err := decoder98.Decode(&h98); err == nil {
		t.Value = h98
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [ImplementationOptions ImplementationRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_inlayHintProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InlayHintOptions:
		return json.Marshal(x)
	case InlayHintRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InlayHintOptions InlayHintRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_inlayHintProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder169 := json.NewDecoder(bytes.NewReader(x))
	decoder169.DisallowUnknownFields()
	var boolVal bool
	if err := decoder169.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder170 := json.NewDecoder(bytes.NewReader(x))
	decoder170.DisallowUnknownFields()
	var h170 InlayHintOptions
	if err := decoder170.Decode(&h170); err == nil {
		t.Value = h170
		return nil
	}
	decoder171 := json.NewDecoder(bytes.NewReader(x))
	decoder171.DisallowUnknownFields()
	var h171 InlayHintRegistrationOptions
	if err := decoder171.Decode(&h171); err == nil {
		t.Value = h171
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InlayHintOptions InlayHintRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_inlineCompletionProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InlineCompletionOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InlineCompletionOptions bool]", t)
}

func (t *Or_ServerCapabilities_inlineCompletionProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder177 := json.NewDecoder(bytes.NewReader(x))
	decoder177.DisallowUnknownFields()
	var boolVal bool
	if err := decoder177.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder178 := json.NewDecoder(bytes.NewReader(x))
	decoder178.DisallowUnknownFields()
	var h178 InlineCompletionOptions
	if err := decoder178.Decode(&h178); err == nil {
		t.Value = h178
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InlineCompletionOptions bool]"}
}

func (t Or_ServerCapabilities_inlineValueProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case InlineValueOptions:
		return json.Marshal(x)
	case InlineValueRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [InlineValueOptions InlineValueRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_inlineValueProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder164 := json.NewDecoder(bytes.NewReader(x))
	decoder164.DisallowUnknownFields()
	var boolVal bool
	if err := decoder164.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder165 := json.NewDecoder(bytes.NewReader(x))
	decoder165.DisallowUnknownFields()
	var h165 InlineValueOptions
	if err := decoder165.Decode(&h165); err == nil {
		t.Value = h165
		return nil
	}
	decoder166 := json.NewDecoder(bytes.NewReader(x))
	decoder166.DisallowUnknownFields()
	var h166 InlineValueRegistrationOptions
	if err := decoder166.Decode(&h166); err == nil {
		t.Value = h166
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [InlineValueOptions InlineValueRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_linkedEditingRangeProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case LinkedEditingRangeOptions:
		return json.Marshal(x)
	case LinkedEditingRangeRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [LinkedEditingRangeOptions LinkedEditingRangeRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_linkedEditingRangeProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder145 := json.NewDecoder(bytes.NewReader(x))
	decoder145.DisallowUnknownFields()
	var boolVal bool
	if err := decoder145.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder146 := json.NewDecoder(bytes.NewReader(x))
	decoder146.DisallowUnknownFields()
	var h146 LinkedEditingRangeOptions
	if err := decoder146.Decode(&h146); err == nil {
		t.Value = h146
		return nil
	}
	decoder147 := json.NewDecoder(bytes.NewReader(x))
	decoder147.DisallowUnknownFields()
	var h147 LinkedEditingRangeRegistrationOptions
	if err := decoder147.Decode(&h147); err == nil {
		t.Value = h147
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [LinkedEditingRangeOptions LinkedEditingRangeRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_monikerProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MonikerOptions:
		return json.Marshal(x)
	case MonikerRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MonikerOptions MonikerRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_monikerProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder154 := json.NewDecoder(bytes.NewReader(x))
	decoder154.DisallowUnknownFields()
	var boolVal bool
	if err := decoder154.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder155 := json.NewDecoder(bytes.NewReader(x))
	decoder155.DisallowUnknownFields()
	var h155 MonikerOptions
	if err := decoder155.Decode(&h155); err == nil {
		t.Value = h155
		return nil
	}
	decoder156 := json.NewDecoder(bytes.NewReader(x))
	decoder156.DisallowUnknownFields()
	var h156 MonikerRegistrationOptions
	if err := decoder156.Decode(&h156); err == nil {
		t.Value = h156
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MonikerOptions MonikerRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_notebookDocumentSync) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case NotebookDocumentSyncOptions:
		return json.Marshal(x)
	case NotebookDocumentSyncRegistrationOptions:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [NotebookDocumentSyncOptions NotebookDocumentSyncRegistrationOptions]", t)
}

func (t *Or_ServerCapabilities_notebookDocumentSync) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder76 := json.NewDecoder(bytes.NewReader(x))
	decoder76.DisallowUnknownFields()
	var h76 NotebookDocumentSyncOptions
	if err := decoder76.Decode(&h76); err == nil {
		t.Value = h76
		return nil
	}
	decoder77 := json.NewDecoder(bytes.NewReader(x))
	decoder77.DisallowUnknownFields()
	var h77 NotebookDocumentSyncRegistrationOptions
	if err := decoder77.Decode(&h77); err == nil {
		t.Value = h77
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [NotebookDocumentSyncOptions NotebookDocumentSyncRegistrationOptions]"}
}

func (t Or_ServerCapabilities_referencesProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case ReferenceOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [ReferenceOptions bool]", t)
}

func (t *Or_ServerCapabilities_referencesProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder100 := json.NewDecoder(bytes.NewReader(x))
	decoder100.DisallowUnknownFields()
	var boolVal bool
	if err := decoder100.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder101 := json.NewDecoder(bytes.NewReader(x))
	decoder101.DisallowUnknownFields()
	var h101 ReferenceOptions
	if err := decoder101.Decode(&h101); err == nil {
		t.Value = h101
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [ReferenceOptions bool]"}
}

func (t Or_ServerCapabilities_renameProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case RenameOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [RenameOptions bool]", t)
}

func (t *Or_ServerCapabilities_renameProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder126 := json.NewDecoder(bytes.NewReader(x))
	decoder126.DisallowUnknownFields()
	var boolVal bool
	if err := decoder126.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder127 := json.NewDecoder(bytes.NewReader(x))
	decoder127.DisallowUnknownFields()
	var h127 RenameOptions
	if err := decoder127.Decode(&h127); err == nil {
		t.Value = h127
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [RenameOptions bool]"}
}

func (t Or_ServerCapabilities_selectionRangeProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case SelectionRangeOptions:
		return json.Marshal(x)
	case SelectionRangeRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [SelectionRangeOptions SelectionRangeRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_selectionRangeProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder135 := json.NewDecoder(bytes.NewReader(x))
	decoder135.DisallowUnknownFields()
	var boolVal bool
	if err := decoder135.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder136 := json.NewDecoder(bytes.NewReader(x))
	decoder136.DisallowUnknownFields()
	var h136 SelectionRangeOptions
	if err := decoder136.Decode(&h136); err == nil {
		t.Value = h136
		return nil
	}
	decoder137 := json.NewDecoder(bytes.NewReader(x))
	decoder137.DisallowUnknownFields()
	var h137 SelectionRangeRegistrationOptions
	if err := decoder137.Decode(&h137); err == nil {
		t.Value = h137
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [SelectionRangeOptions SelectionRangeRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_semanticTokensProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case SemanticTokensOptions:
		return json.Marshal(x)
	case SemanticTokensRegistrationOptions:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [SemanticTokensOptions SemanticTokensRegistrationOptions]", t)
}

func (t *Or_ServerCapabilities_semanticTokensProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder150 := json.NewDecoder(bytes.NewReader(x))
	decoder150.DisallowUnknownFields()
	var h150 SemanticTokensOptions
	if err := decoder150.Decode(&h150); err == nil {
		t.Value = h150
		return nil
	}
	decoder151 := json.NewDecoder(bytes.NewReader(x))
	decoder151.DisallowUnknownFields()
	var h151 SemanticTokensRegistrationOptions
	if err := decoder151.Decode(&h151); err == nil {
		t.Value = h151
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [SemanticTokensOptions SemanticTokensRegistrationOptions]"}
}

func (t Or_ServerCapabilities_textDocumentSync) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TextDocumentSyncKind:
		return json.Marshal(x)
	case TextDocumentSyncOptions:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TextDocumentSyncKind TextDocumentSyncOptions]", t)
}

func (t *Or_ServerCapabilities_textDocumentSync) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder72 := json.NewDecoder(bytes.NewReader(x))
	decoder72.DisallowUnknownFields()
	var h72 TextDocumentSyncKind
	if err := decoder72.Decode(&h72); err == nil {
		t.Value = h72
		return nil
	}
	decoder73 := json.NewDecoder(bytes.NewReader(x))
	decoder73.DisallowUnknownFields()
	var h73 TextDocumentSyncOptions
	if err := decoder73.Decode(&h73); err == nil {
		t.Value = h73
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TextDocumentSyncKind TextDocumentSyncOptions]"}
}

func (t Or_ServerCapabilities_typeDefinitionProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TypeDefinitionOptions:
		return json.Marshal(x)
	case TypeDefinitionRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TypeDefinitionOptions TypeDefinitionRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_typeDefinitionProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder91 := json.NewDecoder(bytes.NewReader(x))
	decoder91.DisallowUnknownFields()
	var boolVal bool
	if err := decoder91.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder92 := json.NewDecoder(bytes.NewReader(x))
	decoder92.DisallowUnknownFields()
	var h92 TypeDefinitionOptions
	if err := decoder92.Decode(&h92); err == nil {
		t.Value = h92
		return nil
	}
	decoder93 := json.NewDecoder(bytes.NewReader(x))
	decoder93.DisallowUnknownFields()
	var h93 TypeDefinitionRegistrationOptions
	if err := decoder93.Decode(&h93); err == nil {
		t.Value = h93
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TypeDefinitionOptions TypeDefinitionRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_typeHierarchyProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TypeHierarchyOptions:
		return json.Marshal(x)
	case TypeHierarchyRegistrationOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TypeHierarchyOptions TypeHierarchyRegistrationOptions bool]", t)
}

func (t *Or_ServerCapabilities_typeHierarchyProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder159 := json.NewDecoder(bytes.NewReader(x))
	decoder159.DisallowUnknownFields()
	var boolVal bool
	if err := decoder159.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder160 := json.NewDecoder(bytes.NewReader(x))
	decoder160.DisallowUnknownFields()
	var h160 TypeHierarchyOptions
	if err := decoder160.Decode(&h160); err == nil {
		t.Value = h160
		return nil
	}
	decoder161 := json.NewDecoder(bytes.NewReader(x))
	decoder161.DisallowUnknownFields()
	var h161 TypeHierarchyRegistrationOptions
	if err := decoder161.Decode(&h161); err == nil {
		t.Value = h161
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TypeHierarchyOptions TypeHierarchyRegistrationOptions bool]"}
}

func (t Or_ServerCapabilities_workspaceSymbolProvider) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case WorkspaceSymbolOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [WorkspaceSymbolOptions bool]", t)
}

func (t *Or_ServerCapabilities_workspaceSymbolProvider) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder117 := json.NewDecoder(bytes.NewReader(x))
	decoder117.DisallowUnknownFields()
	var boolVal bool
	if err := decoder117.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder118 := json.NewDecoder(bytes.NewReader(x))
	decoder118.DisallowUnknownFields()
	var h118 WorkspaceSymbolOptions
	if err := decoder118.Decode(&h118); err == nil {
		t.Value = h118
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [WorkspaceSymbolOptions bool]"}
}

func (t Or_SignatureInformation_documentation) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case MarkupContent:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [MarkupContent string]", t)
}

func (t *Or_SignatureInformation_documentation) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder186 := json.NewDecoder(bytes.NewReader(x))
	decoder186.DisallowUnknownFields()
	var stringVal string
	if err := decoder186.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	decoder187 := json.NewDecoder(bytes.NewReader(x))
	decoder187.DisallowUnknownFields()
	var h187 MarkupContent
	if err := decoder187.Decode(&h187); err == nil {
		t.Value = h187
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [MarkupContent string]"}
}

func (t Or_TextDocumentContentChangeEvent) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TextDocumentContentChangePartial:
		return json.Marshal(x)
	case TextDocumentContentChangeWholeDocument:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TextDocumentContentChangePartial TextDocumentContentChangeWholeDocument]", t)
}

func (t *Or_TextDocumentContentChangeEvent) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder263 := json.NewDecoder(bytes.NewReader(x))
	decoder263.DisallowUnknownFields()
	var h263 TextDocumentContentChangePartial
	if err := decoder263.Decode(&h263); err == nil {
		t.Value = h263
		return nil
	}
	decoder264 := json.NewDecoder(bytes.NewReader(x))
	decoder264.DisallowUnknownFields()
	var h264 TextDocumentContentChangeWholeDocument
	if err := decoder264.Decode(&h264); err == nil {
		t.Value = h264
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TextDocumentContentChangePartial TextDocumentContentChangeWholeDocument]"}
}

func (t Or_TextDocumentEdit_edits_Elem) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case AnnotatedTextEdit:
		return json.Marshal(x)
	case SnippetTextEdit:
		return json.Marshal(x)
	case TextEdit:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [AnnotatedTextEdit SnippetTextEdit TextEdit]", t)
}

func (t *Or_TextDocumentEdit_edits_Elem) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder52 := json.NewDecoder(bytes.NewReader(x))
	decoder52.DisallowUnknownFields()
	var h52 AnnotatedTextEdit
	if err := decoder52.Decode(&h52); err == nil {
		t.Value = h52
		return nil
	}
	decoder53 := json.NewDecoder(bytes.NewReader(x))
	decoder53.DisallowUnknownFields()
	var h53 SnippetTextEdit
	if err := decoder53.Decode(&h53); err == nil {
		t.Value = h53
		return nil
	}
	decoder54 := json.NewDecoder(bytes.NewReader(x))
	decoder54.DisallowUnknownFields()
	var h54 TextEdit
	if err := decoder54.Decode(&h54); err == nil {
		t.Value = h54
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [AnnotatedTextEdit SnippetTextEdit TextEdit]"}
}

func (t Or_TextDocumentFilter) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TextDocumentFilterLanguage:
		return json.Marshal(x)
	case TextDocumentFilterPattern:
		return json.Marshal(x)
	case TextDocumentFilterScheme:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TextDocumentFilterLanguage TextDocumentFilterPattern TextDocumentFilterScheme]", t)
}

func (t *Or_TextDocumentFilter) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder279 := json.NewDecoder(bytes.NewReader(x))
	decoder279.DisallowUnknownFields()
	var h279 TextDocumentFilterLanguage
	if err := decoder279.Decode(&h279); err == nil {
		t.Value = h279
		return nil
	}
	decoder280 := json.NewDecoder(bytes.NewReader(x))
	decoder280.DisallowUnknownFields()
	var h280 TextDocumentFilterPattern
	if err := decoder280.Decode(&h280); err == nil {
		t.Value = h280
		return nil
	}
	decoder281 := json.NewDecoder(bytes.NewReader(x))
	decoder281.DisallowUnknownFields()
	var h281 TextDocumentFilterScheme
	if err := decoder281.Decode(&h281); err == nil {
		t.Value = h281
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TextDocumentFilterLanguage TextDocumentFilterPattern TextDocumentFilterScheme]"}
}

func (t Or_TextDocumentSyncOptions_save) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case SaveOptions:
		return json.Marshal(x)
	case bool:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [SaveOptions bool]", t)
}

func (t *Or_TextDocumentSyncOptions_save) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder195 := json.NewDecoder(bytes.NewReader(x))
	decoder195.DisallowUnknownFields()
	var boolVal bool
	if err := decoder195.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder196 := json.NewDecoder(bytes.NewReader(x))
	decoder196.DisallowUnknownFields()
	var h196 SaveOptions
	if err := decoder196.Decode(&h196); err == nil {
		t.Value = h196
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [SaveOptions bool]"}
}

func (t Or_WorkspaceDocumentDiagnosticReport) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case WorkspaceFullDocumentDiagnosticReport:
		return json.Marshal(x)
	case WorkspaceUnchangedDocumentDiagnosticReport:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [WorkspaceFullDocumentDiagnosticReport WorkspaceUnchangedDocumentDiagnosticReport]", t)
}

func (t *Or_WorkspaceDocumentDiagnosticReport) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder259 := json.NewDecoder(bytes.NewReader(x))
	decoder259.DisallowUnknownFields()
	var h259 WorkspaceFullDocumentDiagnosticReport
	if err := decoder259.Decode(&h259); err == nil {
		t.Value = h259
		return nil
	}
	decoder260 := json.NewDecoder(bytes.NewReader(x))
	decoder260.DisallowUnknownFields()
	var h260 WorkspaceUnchangedDocumentDiagnosticReport
	if err := decoder260.Decode(&h260); err == nil {
		t.Value = h260
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [WorkspaceFullDocumentDiagnosticReport WorkspaceUnchangedDocumentDiagnosticReport]"}
}

func (t Or_WorkspaceEdit_documentChanges_Elem) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case CreateFile:
		return json.Marshal(x)
	case DeleteFile:
		return json.Marshal(x)
	case RenameFile:
		return json.Marshal(x)
	case TextDocumentEdit:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [CreateFile DeleteFile RenameFile TextDocumentEdit]", t)
}

func (t *Or_WorkspaceEdit_documentChanges_Elem) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder4 := json.NewDecoder(bytes.NewReader(x))
	decoder4.DisallowUnknownFields()
	var h4 CreateFile
	if err := decoder4.Decode(&h4); err == nil {
		t.Value = h4
		return nil
	}
	decoder5 := json.NewDecoder(bytes.NewReader(x))
	decoder5.DisallowUnknownFields()
	var h5 DeleteFile
	if err := decoder5.Decode(&h5); err == nil {
		t.Value = h5
		return nil
	}
	decoder6 := json.NewDecoder(bytes.NewReader(x))
	decoder6.DisallowUnknownFields()
	var h6 RenameFile
	if err := decoder6.Decode(&h6); err == nil {
		t.Value = h6
		return nil
	}
	decoder7 := json.NewDecoder(bytes.NewReader(x))
	decoder7.DisallowUnknownFields()
	var h7 TextDocumentEdit
	if err := decoder7.Decode(&h7); err == nil {
		t.Value = h7
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [CreateFile DeleteFile RenameFile TextDocumentEdit]"}
}

func (t Or_WorkspaceFoldersServerCapabilities_changeNotifications) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case bool:
		return json.Marshal(x)
	case string:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [bool string]", t)
}

func (t *Or_WorkspaceFoldersServerCapabilities_changeNotifications) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder210 := json.NewDecoder(bytes.NewReader(x))
	decoder210.DisallowUnknownFields()
	var boolVal bool
	if err := decoder210.Decode(&boolVal); err == nil {
		t.Value = boolVal
		return nil
	}
	decoder211 := json.NewDecoder(bytes.NewReader(x))
	decoder211.DisallowUnknownFields()
	var stringVal string
	if err := decoder211.Decode(&stringVal); err == nil {
		t.Value = stringVal
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [bool string]"}
}

func (t Or_WorkspaceOptions_textDocumentContent) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case TextDocumentContentOptions:
		return json.Marshal(x)
	case TextDocumentContentRegistrationOptions:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [TextDocumentContentOptions TextDocumentContentRegistrationOptions]", t)
}

func (t *Or_WorkspaceOptions_textDocumentContent) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder199 := json.NewDecoder(bytes.NewReader(x))
	decoder199.DisallowUnknownFields()
	var h199 TextDocumentContentOptions
	if err := decoder199.Decode(&h199); err == nil {
		t.Value = h199
		return nil
	}
	decoder200 := json.NewDecoder(bytes.NewReader(x))
	decoder200.DisallowUnknownFields()
	var h200 TextDocumentContentRegistrationOptions
	if err := decoder200.Decode(&h200); err == nil {
		t.Value = h200
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [TextDocumentContentOptions TextDocumentContentRegistrationOptions]"}
}

func (t Or_WorkspaceSymbol_location) MarshalJSON() ([]byte, error) {
	switch x := t.Value.(type) {
	case Location:
		return json.Marshal(x)
	case LocationUriOnly:
		return json.Marshal(x)
	case nil:
		return []byte("null"), nil
	}
	return nil, fmt.Errorf("type %T not one of [Location LocationUriOnly]", t)
}

func (t *Or_WorkspaceSymbol_location) UnmarshalJSON(x []byte) error {
	if string(x) == "null" {
		t.Value = nil
		return nil
	}
	decoder39 := json.NewDecoder(bytes.NewReader(x))
	decoder39.DisallowUnknownFields()
	var h39 Location
	if err := decoder39.Decode(&h39); err == nil {
		t.Value = h39
		return nil
	}
	decoder40 := json.NewDecoder(bytes.NewReader(x))
	decoder40.DisallowUnknownFields()
	var h40 LocationUriOnly
	if err := decoder40.Decode(&h40); err == nil {
		t.Value = h40
		return nil
	}
	return &UnmarshalError{"unmarshal failed to match one of [Location LocationUriOnly]"}
}
