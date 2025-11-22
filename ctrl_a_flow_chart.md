# Ctrl+A Implementation Flow Chart

## Overview
This document illustrates how Ctrl+A (Select All) works in the Crush TUI application, which has two distinct contexts where Ctrl+A can be triggered:

1. **Editor Component** - Selects all text in the input field
2. **Chat Messages Component** - Selects all messages in the chat list

## Architecture Flow Chart

```mermaid
flowchart TD
    A[User Presses Ctrl+A/Cmd+A] --> B{Focus Detection}
    
    B -->|Editor Focused| C[Editor Ctrl+A Path]
    B -->|Chat Focused| D[Chat Ctrl+A Path]
    
    %% Editor Ctrl+A Flow
    subgraph "Editor Ctrl+A Implementation"
        C --> E[key_handlers.go: Line 21]
        E --> F["key.Matches(msg, m.keyMap.SelectAll)?"]
        F -->|Yes| G[Call m.SelectAll]
        F -->|No| H[Continue to other handlers]
        
        G --> I[editor.go: Line 502-504]
        I --> J["m.selection.SelectAll"]
        
        J --> K[selection.go: Line 102-112]
        K --> L["Get textarea content"]
        L --> M{"Content empty?"}
        M -->|Yes| N[Clear selection]
        M -->|No| O["Set selection bounds: 0 to len([]rune(content))"]
        
        O --> P[Render selection highlighting]
        P --> Q[rendering.go: Line 11-39]
        Q --> R["Apply TextSelection style"]
        R --> S["User sees highlighted text"]
        
        H --> T[isHandledKey prevents textarea override]
    end
    
    %% Chat Ctrl+A Flow
    subgraph "Chat Ctrl+A Implementation"
        D --> U[chat.go: Line 125-127]
        U --> V["key.Matches(msg, messages.SelectAllKey)?"]
        V -->|Yes| W[Call m.SelectAll]
        V -->|No| X[Continue to other handlers]
        
        W --> Y[chat.go: Line 812-839]
        Y --> Z{"Any messages?"}
        Z -->|No| AA[Return warning: No messages]
        Z -->|Yes| BB[Set first item as selected]
        
        BB --> CC[Get list dimensions]
        CC --> DD["StartSelection(0, 0)"]
        DD --> EE["EndSelection(width-1, height*10)"]
        
        EE --> FF[Scroll to selection]
        FF --> GG[Apply highlighting to messages]
        GG --> HH["User sees all messages selected"]
        
        X --> II[Continue with other key handling]
    end
    
    %% Shared Components
    subgraph "Shared Key Binding Infrastructure"
        JJ[keys.go: Line 38-41]
        JJ --> KK["SelectAll: key.NewBinding(ctrl+a, cmd+a)"]
        
        LL[messages.go: Line 32-33]
        LL --> MM["SelectAllKey: key.NewBinding(ctrl+a, cmd+a)"]
    end
    
    %% Styling and Visualization
    subgraph "Selection Rendering System"
        NN[Selection Manager] 
        NN --> OO[Unicode-safe text handling]
        NN --> PP[Bounds validation]
        NN --> QQ[Style application]
        
        QQ --> RR[lipgloss styling]
        RR --> SS[TextSelection theme style]
        SS --> TT[Visual feedback to user]
    end
    
    %% Testing Infrastructure
    subgraph "Comprehensive Test Coverage"
        UU[ctrl_a_test.go] --> VV[Test key binding recognition]
        WW[selection_test.go] --> XX[Test selection logic]
        YY[integration_test.go] --> ZZ[Test end-to-end flow]
        AAA[benchmark_test.go] --> BBB[Test performance]
    end
    
    %% Connections to shared components
    E -.-> JJ
    V -.-> LL
    K -.-> NN
    O -.-> NN
    GG -.-> NN
    
    %% Test connections
    G -.-> UU
    J -.-> WW
    W -.-> YY
    K -.-> AAA
    
    %% Styling connections
    Q -.-> NN
    GG -.-> NN
```

## Detailed Component Analysis

### 1. Key Detection and Routing

```mermaid
sequenceDiagram
    participant User
    participant TUI as Bubble Tea
    participant Router as Key Router
    participant Editor as Editor Component
    participant Chat as Chat Component
    
    User->>TUI: Presses Ctrl+A
    TUI->>Router: tea.KeyPressMsg
    Router->>Router: Check focus state
    alt Editor Focused
        Router->>Editor: Forward to editor
        Editor->>Editor: handleSelectionKeyBindings()
    else Chat Focused
        Router->>Chat: Forward to chat
        Chat->>Chat: Process SelectAllKey
    end
```

### 2. Editor Select All Implementation

```mermaid
flowchart TD
    A[Ctrl+A in Editor] --> B[key_handlers.go:21]
    B --> C{"key.Matches?"}
    C -->|Yes| D[SelectAll call]
    C -->|No| E[Continue processing]
    
    D --> F[editor.go:502]
    F --> G[selection.SelectAll]
    
    G --> H[selection.go:102]
    H --> I[Get textarea content]
    I --> J{"Empty?"}
    J -->|Yes| K[Clear selection]
    J -->|No| L[Set bounds 0→len runes]
    
    L --> M[Mark as inactive]
    M --> N[Ready for copying]
    
    N --> O[Rendering system]
    O --> P[Apply highlighting]
    P --> Q[Display to user]
    
    E --> R[isHandledKey = true]
    R --> S[Prevent textarea override]
```

### 3. Chat Select All Implementation

```mermaid
flowchart TD
    A[Ctrl+A in Chat] --> B[chat.go:125]
    B --> C{"key.Matches SelectAllKey?"}
    C -->|Yes| D[SelectAll command]
    C -->|No| E[Continue processing]
    
    D --> F[chat.go:812]
    F --> G{"Messages exist?"}
    G -->|No| H[Return warning]
    G -->|Yes| I[Select first item]
    
    I --> J[Get dimensions]
    J --> K[StartSelection 0,0]
    K --> L[EndSelection width-1, height*10]
    
    L --> M[List component clamps bounds]
    M --> N[Select all items]
    N --> O[Apply selection styling]
    O --> P[User sees all messages selected]
    
    E --> Q[Other key handling]
```

## Key Design Principles

### 1. Unicode Safety
- All text operations use `[]rune` instead of byte indices
- Proper handling of multi-byte Unicode characters
- Selection bounds work with character positions, not byte positions

### 2. Component Isolation
- Editor and Chat handle SelectAll independently
- Each component maintains its own selection state
- Shared styling system but independent logic

### 3. Performance Considerations
- Lazy rendering of selection highlights
- Efficient bounds validation
- Minimal state updates

### 4. Cross-Platform Compatibility
- Supports both `ctrl+a` (Windows/Linux) and `cmd+a` (macOS)
- Terminal-agnostic implementation
- Graceful fallback for unsupported terminals

## Recent Improvements (Based on Git History)

1. **True Select All for Chat** - Changed from `height - 1` to `height * 10` to include all scrollable content
2. **Terminal Behavior Override** - Prevents terminal-wide selection, focuses on input field only
3. **Unicode Support** - Proper rune-count based selection for international characters
4. **Enhanced Testing** - Comprehensive BDD tests with proper mock handling

## Error Handling

```mermaid
flowchart TD
    A[Selection Operation] --> B{"Valid content?"}
    B -->|No| C[Clear selection silently]
    B -->|Yes| D{"Valid bounds?"}
    D -->|No| E[Clear selection]
    D -->|Yes| F[Apply selection]
    
    F --> G{"Rendering successful?"}
    G -->|No| H[Fallback to no highlighting]
    G -->|Yes| I[Display selection]
    
    C --> J[User sees no selection]
    E --> J
    H --> J
    I --> K[User sees highlighted text]
```

## Testing Strategy

The implementation includes comprehensive testing:

1. **Unit Tests** - Individual component behavior
2. **Integration Tests** - End-to-end key handling flow
3. **BDD Tests** - Behavior-driven scenarios
4. **Performance Tests** - Benchmark selection operations
5. **Cross-Platform Tests** - Verify key bindings work on different platforms

This robust testing ensures the Ctrl+A functionality works reliably across all supported environments and use cases.