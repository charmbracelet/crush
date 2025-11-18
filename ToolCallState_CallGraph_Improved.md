# ToolCallState Call Graph - Improved Implementation

## Improved Implementation Design

The improved `ToolCallState` implementation addresses all architectural issues through separation of concerns and clean architecture principles.

### Architecture Principles Applied:
1. **Single Responsibility** - Each component has one clear purpose
2. **Separation of Concerns** - State logic separate from UI rendering
3. **Dependency Inversion** - State doesn't depend on UI packages
4. **Explicit Transitions** - Clear state machine with validation
5. **Composability** - Separate concerns can be combined

### Improved State Structure:
```mermaid
graph TD
    A[ToolCallState Composite] --> B[ExecutionState]
    A --> C[PermissionState]
    
    B --> B1[Pending]
    B --> B2[Running]
    B --> B3[Completed]
    B --> B4[Failed]
    B --> B5[Cancelled]
    
    C --> C1[NotRequired]
    C --> C2[Requested]
    C --> C3[Approved]
    C --> C4[Denied]
    
    A --> D[State Methods]
    D --> D1[IsFinal]
    D --> D2[CanTransition]
    D --> D3[String]
    D --> D4[Equals]
    
    E[State Validators] --> E1[TransitionValidator]
    E --> E2[StateInvariantChecker]
    
    F[State Transitions] --> F1[ValidTransitions Matrix]
    F --> F2[TransitionEvents]
    
    G[UI Mappers] --> G1[ToolCallUIMapper]
    G --> G2[AnimationStateMapper]
    G --> G3[ContentVisibilityMapper]
    
    H[External Dependencies] --> H1[styles package]
    H --> H2[anim package]
    
    G --> H
    
    style A fill:#99ff99
    style D fill:#ccffcc
    style E fill:#ccffcc
    style F fill:#ccffcc
    style G fill:#ffffcc
    style H fill:#ffe6e6
```

### Improved Class Hierarchy:
```mermaid
classDiagram
    class ToolCallState {
        -ExecutionState execution
        -PermissionState permission
        +New(execution, permission) ToolCallState
        +Execution() ExecutionState
        +Permission() PermissionState
        +IsFinal() bool
        +CanTransition(to ToolCallState) bool
        +String() string
        +Equals(other ToolCallState) bool
    }
    
    class ExecutionState {
        <<enumeration>>
        PENDING
        RUNNING
        COMPLETED
        FAILED
        CANCELLED
        +IsFinal() bool
        +String() string
    }
    
    class PermissionState {
        <<enumeration>>
        NOT_REQUIRED
        REQUESTED
        APPROVED
        DENIED
        +IsFinal() bool
        +RequiresApproval() bool
        +String() string
    }
    
    class StateTransitionValidator {
        +Validate(from, to ToolCallState) error
        +GetValidTransitions(from ToolCallState) []ToolCallState
        +IsValidTransition(from, to ToolCallState) bool
    }
    
    class ToolCallUIMapper {
        +ToIcon(state ToolCallState) string
        +ToFgColor(state ToolCallState) color.Color
        +ToMessage(state ToolCallState) string
        +ToColoredMessage(state ToolCallState) string
        +ToCopyFormat(state ToolCallState) string
    }
    
    class AnimationStateMapper {
        +ToAnimationState(state ToolCallState) AnimationState
        +ToAnimationSettings(state ToolCallState, isNested bool) anim.Settings
    }
    
    class ContentVisibilityMapper {
        +ShouldShowContent(state ToolCallState, isNested, hasNested bool) bool
    }
    
    ToolCallState --> ExecutionState
    ToolCallState --> PermissionState
    StateTransitionValidator --> ToolCallState
    ToolCallUIMapper --> ToolCallState
    AnimationStateMapper --> ToolCallState
    ContentVisibilityMapper --> ToolCallState
```

### Improved State Machine:
```mermaid
stateDiagram-v2
    [*] --> InitialState
    
    state "Composite States" as CS {
        state "Execution State" as ES {
            [*] --> Pending
            Pending --> Running
            Running --> Completed
            Running --> Failed
            Pending --> Failed
            Completed --> [*]
            Failed --> [*]
        }
        
        state "Permission State" as PS {
            [*] --> NotRequired
            NotRequired --> Requested
            Requested --> Approved
            Requested --> Denied
            Approved --> [*]
            Denied --> [*]
        }
    }
    
    InitialState --> Composite: Create
    
    Composite --> FinalState: IsFinal()
    FinalState --> [*]
```

### Improved Dependency Flow:
```mermaid
graph LR
    A[Business Logic] --> B[State Layer]
    B --> C[UI Mapper Layer]
    C --> D[UI/Presentation Layer]
    
    E[Agent] --> F[ToolCall]
    F --> G[ToolCallState]
    G --> H[StateTransitionValidator]
    
    I[UI Components] --> J[ToolCallUIMapper]
    I --> K[AnimationStateMapper]
    I --> L[ContentVisibilityMapper]
    
    J --> M[styles package]
    K --> N[anim package]
    
    G --> J
    G --> K
    G --> L
    
    style B fill:#ccffcc
    style C fill:#ffffcc
    style D fill:#ffcccc
    style G fill:#99ff99
    style J fill:#ffffcc
    style K fill:#ffffcc
    style L fill:#ffffcc
```

### Improved State Transition Matrix:
```mermaid
graph TD
    A[State Transition Matrix] --> B[From: Pending]
    A --> C[From: PermissionRequested]
    A --> D[From: Running]
    
    B --> B1[To: Running ✓]
    B --> B2[To: Failed ✓]
    B --> B3[To: Cancelled ✓]
    B --> B4[To: Completed ✗]
    
    C --> C1[To: Approved ✓]
    C --> C2[To: Denied ✓]
    C --> C3[To: Cancelled ✓]
    C --> C4[To: Running ✗]
    
    D --> D1[To: Completed ✓]
    D --> D2[To: Failed ✓]
    D --> D3[To: Cancelled ✓]
    D --> D4[To: Pending ✗]
    
    style A fill:#ccffcc
    style B1 fill:#99ff99
    style B2 fill:#99ff99
    style B3 fill:#99ff99
    style B4 fill:#ffcccc
    style C1 fill:#99ff99
    style C2 fill:#99ff99
    style C3 fill:#99ff99
    style C4 fill:#ffcccc
    style D1 fill:#99ff99
    style D2 fill:#99ff99
    style D3 fill:#99ff99
    style D4 fill:#ffcccc
```

### Improved Usage Pattern:
```mermaid
sequenceDiagram
    participant Agent
    participant ToolCall
    participant ToolCallState
    participant StateValidator
    participant UIMapper
    participant AnimationMapper
    participant UI
    participant AnimationSystem
    
    Agent->>ToolCall: Create with execution/permission
    ToolCall->>ToolCallState: New(execution, permission)
    
    Agent->>ToolCall: Request state change
    ToolCall->>StateValidator: ValidateTransition(from, to)
    StateValidator-->>ToolCall: validation result
    
    alt Valid Transition
        ToolCall->>ToolCallState: UpdateState(newState)
    end
    
    UI->>UIMapper: ToIcon(state)
    UIMapper->>UIMapper: MapExecutionIcon(execution)
    UIMapper->>UIMapper: MapPermissionIcon(permission)
    UIMapper-->>UI: combined icon
    
    UI->>UIMapper: ToColoredMessage(state)
    UIMapper-->>UI: styled message
    
    ToolCall->>AnimationMapper: ToAnimationSettings(state, isNested)
    AnimationMapper-->>ToolCall: anim.Settings
    
    ToolCall->>AnimationSystem: Update with settings
    AnimationSystem-->>ToolCall: animation frame
```

### Improved Error Handling:
```mermaid
graph TD
    A[State Transition Attempt] --> B{Valid Transition?}
    
    B -->|Yes| C[Execute Transition]
    B -->|No| D[Return Error]
    
    C --> E{State Invariant Valid?}
    
    E -->|Yes| F[Update State]
    E -->|No| G[Rollback + Error]
    
    F --> H[Notify Observers]
    H --> I[Success]
    
    D --> J[InvalidTransitionError]
    G --> K[StateInvariantError]
    
    style A fill:#e6f3ff
    style C fill:#ccffcc
    style F fill:#ccffcc
    style H fill:#ccffcc
    style I fill:#99ff99
    style D fill:#ffcccc
    style G fill:#ffcccc
    style J fill:#ff9999
    style K fill:#ff9999
```

### Benefits of Improved Design:
1. **Clean Separation** - State logic isolated from UI concerns
2. **Testability** - Pure state functions, easy to unit test
3. **Extensibility** - New UI styles without changing state
4. **Maintainability** - Single responsibility for each component
5. **Type Safety** - Compile-time validation of transitions
6. **Performance** - No UI package dependencies in state
7. **Flexibility** - Composite states allow complex combinations

### Migration Strategy:
```mermaid
graph LR
    A[Current Implementation] --> B[Add New Classes]
    B --> C[Update Usage Gradually]
    C --> D[Deprecate Old Methods]
    D --> E[Remove Old Implementation]
    
    F[Backward Compatibility] --> G[Adapter Pattern]
    G --> H[Old Methods Call New Implementation]
    H --> I[Gradual Migration]
    
    style A fill:#ffcccc
    style B fill:#ffffcc
    style C fill:#ffffcc
    style D fill:#ffffcc
    style E fill:#ccffcc
    style G fill:#ccffcc
```

This improved design transforms the monolithic state enum into a clean, testable, and maintainable state management system that follows SOLID principles and clean architecture practices.