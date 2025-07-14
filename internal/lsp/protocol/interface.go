package protocol

import "fmt"

// WorkspaceSymbolResult is an interface for types that represent workspace symbols
type WorkspaceSymbolResult interface {
	GetName() string
	GetLocation() Location
	isWorkspaceSymbol() // marker method
}

func (ws *WorkspaceSymbol) GetName() string { return ws.Name }
func (ws *WorkspaceSymbol) GetLocation() Location {
	switch v := ws.Location.Value.(type) {
	case Location:
		return v
	case LocationUriOnly:
		return Location{URI: v.URI}
	}
	return Location{}
}
func (ws *WorkspaceSymbol) isWorkspaceSymbol() {}

func (si *SymbolInformation) GetName() string       { return si.Name }
func (si *SymbolInformation) GetLocation() Location { return si.Location }
func (si *SymbolInformation) isWorkspaceSymbol()    {}

// Results converts the Value to a slice of WorkspaceSymbolResult
func (r Or_Result_workspace_symbol) Results() ([]WorkspaceSymbolResult, error) {
	if r.Value == nil {
		return make([]WorkspaceSymbolResult, 0), nil
	}
	switch v := r.Value.(type) {
	case []WorkspaceSymbol:
		results := make([]WorkspaceSymbolResult, len(v))
		for i := range v {
			results[i] = &v[i]
		}
		return results, nil
	case []SymbolInformation:
		results := make([]WorkspaceSymbolResult, len(v))
		for i := range v {
			results[i] = &v[i]
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unknown symbol type: %T", r.Value)
	}
}

// DocumentSymbolResult is an interface for types that represent document symbols
type DocumentSymbolResult interface {
	GetRange() Range
	GetName() string
	isDocumentSymbol() // marker method
}

func (ds *DocumentSymbol) GetRange() Range   { return ds.Range }
func (ds *DocumentSymbol) GetName() string   { return ds.Name }
func (ds *DocumentSymbol) isDocumentSymbol() {}

func (si *SymbolInformation) GetRange() Range { return si.Location.Range }

// Note: SymbolInformation already has GetName() implemented above
func (si *SymbolInformation) isDocumentSymbol() {}

// Results converts the Value to a slice of DocumentSymbolResult
func (r Or_Result_textDocument_documentSymbol) Results() ([]DocumentSymbolResult, error) {
	if r.Value == nil {
		return make([]DocumentSymbolResult, 0), nil
	}
	switch v := r.Value.(type) {
	case []DocumentSymbol:
		results := make([]DocumentSymbolResult, len(v))
		for i := range v {
			results[i] = &v[i]
		}
		return results, nil
	case []SymbolInformation:
		results := make([]DocumentSymbolResult, len(v))
		for i := range v {
			results[i] = &v[i]
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unknown document symbol type: %T", v)
	}
}

// TextEditResult is an interface for types that can be used as text edits
type TextEditResult interface {
	GetRange() Range
	GetNewText() string
	isTextEdit() // marker method
}

func (te *TextEdit) GetRange() Range    { return te.Range }
func (te *TextEdit) GetNewText() string { return te.NewText }
func (te *TextEdit) isTextEdit()        {}

// AsTextEdit converts Or_TextDocumentEdit_edits_Elem to TextEdit
func (e Or_TextDocumentEdit_edits_Elem) AsTextEdit() (TextEdit, error) {
	if e.Value == nil {
		return TextEdit{}, fmt.Errorf("nil text edit")
	}
	switch v := e.Value.(type) {
	case TextEdit:
		return v, nil
	case AnnotatedTextEdit:
		return TextEdit{
			Range:   v.Range,
			NewText: v.NewText,
		}, nil
	default:
		return TextEdit{}, fmt.Errorf("unknown text edit type: %T", e.Value)
	}
}
