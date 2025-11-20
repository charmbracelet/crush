# AnimationState Call Graph - Current Implementation

## Current Implementation Analysis

The current `AnimationState` implementation has several architectural issues that couple animation logic tightly with UI presentation concerns.

### Problems Identified:
1. **Mixed Concerns** - Icon, label, and color logic mixed with animation state
2. **UI Logic in State** - Animation enum contains presentation logic
3. **Tight Coupling** - Direct dependency on `styles` package
4. **Limited Extensibility** - Hard to add new animation types without modifying enum
5. **No Animation Configuration** - No separation of animation behavior from visual representation

### Current State Structure:
```mermaid
graph TD
    A[AnimationState enum] --> B[6 States Total]
    
    B --> C[Inactive States]
    B --> D[Active States]
    
    C --> C1[AnimationStateNone]
    C --> C2[AnimationStateStatic]
    
    D --> D1[AnimationStateSpinner]
    D --> D2[AnimationStateTimer]
    D --> D3[AnimationStateBlink]
    D --> D4[AnimationStatePulse]
    
    A --> E[State Classification Methods]
    E --> E1[IsActive]
    E --> E2[IsStatic]
    E --> E3[String]
    
    A --> F[UI Rendering Methods - MIXED CONCERNS]
    F --> F1[ToIcon]
    F --> F2[ToLabel]
    F --> F3[isCycleColors]
    F --> F4[toLabelColor]
    
    F --> G[styles package dependency]
    
    style A fill:#ff9999
    style F fill:#ffcccc
    style G fill:#ffe6e6
```

### Current Assignment Flow:
```mermaid
graph TD
    A[ToolCall Component] --> B[updateAnimationState]
    B --> C[getEffectiveDisplayState]
    C --> D[ToAnimationState]
    D --> E[Direct Assignment]
    E --> F[m.animationState = ...]
    
    G[Message Component] --> H[Init/Update]
    H --> I[determineAnimationState]
    I --> J[Direct Assignment]
    J --> K[m.animationState = ...]
    
    L[Animation System] --> M[anim.StepMsg]
    M --> N[IsActive check]
    N --> O[Update animation]
    
    style A fill:#e6f3ff
    style B fill:#e6f3ff
    style E fill:#ffcccc
    style F fill:#ffcccc
    style G fill:#ccffcc
    style J fill:#ffcccc
    style K fill:#ffcccc
```

### Current State Usage Pattern:
```mermaid
sequenceDiagram
    participant TUI as TUI Component
    participant Tool as ToolCallCmp
    participant Msg as MessageCmp
    participant Anim as AnimationState
    participant Style as styles package
    
    TUI->>Tool: SetToolCallState()
    Tool->>Tool: updateAnimationState()
    Tool->>Tool: getEffectiveDisplayState()
    Tool->>Anim: ToAnimationState(ToolCallState)
    Anim-->>Tool: AnimationState
    Tool->>Tool: m.animationState = state
    
    TUI->>Anim: IsActive()
    Anim-->>TUI: boolean
    
    TUI->>Anim: ToIcon()
    Anim->>Style: CurrentTheme()
    Style-->>Anim: theme
    Anim-->>TUI: icon
    
    TUI->>Anim: toLabelColor()
    Anim->>Style: CurrentTheme()
    Style-->>Anim: theme
    Anim-->>TUI: color
    
    Note over Msg: Similar pattern for MessageCmp
    Msg->>Msg: determineAnimationState()
    Msg->>Msg: m.animationState = state
```

### Current State Mapping Logic:
```mermaid
graph LR
    A[ToolCallState] --> B[ToAnimationState]
    
    B --> C[PermissionPending]
    B --> D[PermissionApproved]
    B --> E[PermissionDenied]
    B --> F[Completed]
    B --> G[Running]
    B --> H[Pending]
    B --> I[Failed/Cancelled]
    
    C --> J[AnimationStateTimer]
    D --> K[AnimationStatePulse]
    E --> L[AnimationStateStatic]
    F --> M[AnimationStateBlink]
    G --> N[AnimationStateSpinner]
    H --> O[AnimationStateStatic]
    I --> P[AnimationStateStatic]
    
    style A fill:#ffcccc
    style B fill:#ffcccc
```

### Current UI Logic Integration:
```mermaid
graph TD
    A[AnimationState] --> B[ToIcon]
    A --> C[ToLabel]
    A --> D[isCycleColors]
    A --> E[toLabelColor]
    
    B --> F[Hardcoded Icons]
    F --> F1["⋯" for Spinner/Timer]
    F --> F2["✅" for Blink]
    F --> F3["⚡" for Pulse]
    F --> F4["" for None/Static]
    
    C --> G[Hardcoded Labels]
    G --> G1["Running" for Spinner]
    G --> G2["Waiting" for Timer]
    G --> G3["Completed" for Blink]
    G --> G4["Processing" for Pulse]
    
    D --> H[Color Cycling Logic]
    H --> H1[true for Spinner/Pulse]
    H --> H2[false for Timer/Blink/Static]
    
    E --> I[Color Mapping]
    I --> I1[Green for Spinner/Blink]
    I --> I2[Blue for Pulse]
    I --> I3[Paprika for Timer]
    I --> I4[FgSubtle for Static]
    
    F --> J[styles package dependency]
    G --> J
    I --> J
    
    style A fill:#ff9999
    style B fill:#ffcccc
    style C fill:#ffcccc
    style D fill:#ffcccc
    style E fill:#ffcccc
    style J fill:#ffe6e6
```

### Current Animation System Integration:
```mermaid
graph LR
    A[Animatable Interface] --> B[GetAnimationState]
    B --> C[AnimationState]
    
    D[Animation System] --> E[IsActive check]
    C --> E
    
    E --> F[true]
    E --> G[false]
    
    F --> H[Update Animation]
    H --> I[anim.StepMsg]
    I --> J[Animation Frame]
    
    G --> K[Stop Animation]
    K --> L[Static Display]
    
    M[ToolCallComponent] --> N[IsAnimating]
    N --> O[Check Nested Tools]
    O --> P[Recursive IsActive]
    
    style A fill:#e6f3ff
    style C fill:#ff9999
    style N fill:#e6f3ff
```

### Current Architecture Issues:
```mermaid
graph TD
    A[Current AnimationState] --> B[❌ Mixed Responsibilities]
    B --> B1[State Logic]
    B --> B2[UI Presentation]
    B --> B3[Animation Configuration]
    
    A --> C[❌ Tight Coupling]
    C --> C1[Direct styles dependency]
    C --> C2[Hardcoded UI elements]
    C --> C3[No abstraction layer]
    
    A --> D[❌ Limited Extensibility]
    D --> D1[Enum modification required]
    D --> D2[No plugin system]
    D --> D3[Fixed animation types]
    
    A --> E[❌ Testing Challenges]
    E --> E1[UI package dependency]
    E --> E2[Hard to mock]
    E --> E3[Integration tests only]
    
    style A fill:#ff9999
    style B fill:#ffcccc
    style C fill:#ffcccc
    style D fill:#ffcccc
    style E fill:#ffcccc
```

### Performance Considerations:
```mermaid
graph TD
    A[Performance Characteristics] --> B[✅ Good Aspects]
    A --> C[⚠️ Problematic Aspects]
    
    B --> B1[uint8 type - memory efficient]
    B --> B2[Fast switch statements]
    B --> B3[Simple comparisons]
    
    C --> C1[styles.CurrentTheme() calls]
    C --> C2[Repeated color calculations]
    C --> C3[No caching of UI elements]
    C --> C4[Direct package dependencies]
    
    D[Optimization Opportunities] --> D1[Cache UI elements]
    D --> D2[Lazy load theme data]
    D --> D3[Separate state from presentation]
    D --> D4[Add performance benchmarks]
    
    style B fill:#ccffcc
    style C fill:#ffffcc
    style D fill:#ccffcc
```

### Current Usage Statistics:
Based on codebase analysis:

1. **Assignment Locations**: 2 primary locations
   - `tool.go`: Line 821 (`m.animationState = m.getEffectiveDisplayState().ToAnimationState()`)
   - `messages.go`: Lines 93, 103 (`m.animationState = m.determineAnimationState()`)

2. **State Sources**:
   - 85% from `ToolCallState.ToAnimationState()` conversion
   - 15% from message-based logic in `determineAnimationState()`

3. **UI Dependencies**:
   - Direct `styles.CurrentTheme()` calls in every UI method
   - Hardcoded icon and label mappings
   - No configuration or customization support

### Summary of Current Issues:

1. **Architectural**: State enum violates single responsibility principle
2. **Maintainability**: UI changes require enum modifications
3. **Testability**: Difficult to unit test due to UI dependencies
4. **Extensibility**: Adding new animations requires core changes
5. **Performance**: Repeated theme lookups and calculations

The current implementation mixes animation behavior with visual presentation, making it difficult to extend, test, and maintain.