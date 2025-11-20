# Crush Application Call Flow Analysis

This document provides comprehensive call flow diagrams for the Crush AI assistant application, showing the complete system architecture and interaction patterns.

## Table of Contents

1. [Application Startup Flow](#application-startup-flow)
2. [TUI Event Loop Flow](#tui-event-loop-flow) 
3. [Agent Processing Flow](#agent-processing-flow)
4. [Tool Execution Flow](#tool-execution-flow)
5. [Permission Management Flow](#permission-management-flow)
6. [Message Flow Architecture](#message-flow-architecture)
7. [LSP Integration Flow](#lsp-integration-flow)
8. [Session Management Flow](#session-management-flow)

---

## Application Startup Flow

```mermaid
flowchart TD
    A[main.go] --> B[cmd.Execute]
    B --> C[rootCmd.RunE]
    C --> D[setupAppWithProgressBar]
    D --> E[setupApp]
    E --> F[ResolveCwd]
    E --> G[config.Init]
    E --> H[createDotCrushDir]
    E --> I[db.Connect]
    I --> J[Run Migrations]
    E --> K[app.New]
    K --> L[Create Services]
    K --> M[Setup Events]
    K --> N[Init LSP Clients]
    K --> O[Init MCP Clients]
    K --> P[Init Coder Agent]
    P --> Q[agent.NewCoordinator]
    Q --> R[buildAgent]
    R --> S[buildAgentModels]
    R --> T[buildTools]
    C --> U[tui.New]
    U --> V[chatPage.New]
    U --> W[Setup Dialogs]
    U --> X[Setup Completions]
    C --> Y[tea.NewProgram]
    Y --> Z[program.Run]
    Z --> AA[TUI Event Loop]
    C --> AB[go app.Subscribe]
    AB --> AC[Event Forwarding]
    C --> AD[event.AppInitialized]
    AD --> AE[Cleanup Handlers]
    
    L --> L1[session.Service]
    L --> L2[message.Service]
    L --> L3[history.Service]
    L --> L4[permission.Service]
    L --> L5[csync.Map for LSP]
```

### Startup Sequence Details

The application follows this initialization sequence:

1. **Main Entry Point** (`main.go:13-24`)
   - Sets up profiling if `CRUSH_PROFILE` is enabled
   - Delegates to `cmd.Execute()`

2. **Command Setup** (`cmd/root.go:77-104`)
   - Resolves working directory
   - Initializes configuration
   - Creates database connection
   - Sets up TUI with Bubble Tea

3. **App Service Creation** (`app/app.go:65-117`)
   - Creates core services (sessions, messages, history, permissions)
   - Initializes agent coordinator
   - Sets up LSP and MCP clients
   - Configures event subscriptions

4. **TUI Initialization** (`tui/tui.go:687-708`)
   - Creates chat page and dialogs
   - Sets up key bindings
   - Configures completions system

---

## TUI Event Loop Flow

```mermaid
flowchart TD
    A[tea.Program] --> B[appModel.Update]
    B --> C{Message Type}
    
    C -->|tea.WindowSizeMsg| D[handleWindowResize]
    D --> D1[Update All Components]
    D1 --> D2[status.Update]
    D2 --> D3[pages.Update]
    D3 --> D4[dialog.Update]
    
    C -->|tea.KeyPressMsg| E[handleKeyPressMsg]
    E --> E1{Key Match?}
    E1 -->|Quit| F[Quit Dialog]
    E1 -->|Help| G[Toggle Help]
    E1 -->|Commands| H[Commands Dialog]
    E1 -->|Models| I[Models Dialog]
    E1 -->|Sessions| J[Sessions Dialog]
    E1 -->|Other| K[Forward to Current Page]
    
    C -->|page.PageChangeMsg| L[moveToPage]
    L --> L1[Check if Agent Busy]
    L --> L2[Initialize Page if Needed]
    L --> L3[Set Page Size]
    
    C -->|dialogs.OpenDialogMsg| M[dialog.Update]
    M --> M1[Close Completions]
    M --> M2[Open New Dialog]
    
    C -->|"pubsub.Event[permission.PermissionRequest]"| N[Permission Dialog]
    N --> N1[Create Permission Dialog]
    N --> N2[Show to User]
    
    C -->|commands.SwitchModelMsg| O[Model Switch]
    O --> O1[Check Agent Busy]
    O --> O2[Update Config]
    O --> O3[Update Agent Model]
    
    C -->|pubsub.UpdateAvailableMsg| P[Status Update]
    P --> P1[Show Update Notification]
    
    C -->|Default| Q[Forward to Current Page]
    Q --> Q1["pages[currentPage].Update"]
    Q --> Q2[dialog.Update if Active]
    
    F --> F1[Quit Dialog]
    F1 --> F2{User Confirms?}
    F2 -->|Yes| F3[tea.Quit]
    F2 -->|No| F4[Continue]
    
    G --> G1[Toggle Full Help]
    G --> G2[Resize Layout]
    
    H --> H1[Open Commands Dialog]
    I --> I1[Open Models Dialog]
    J --> J1[Open Sessions Dialog]
```

### TUI Event Processing Details

The TUI uses Bubble Tea's Elm architecture:

1. **Message Distribution** (`tui/tui.go:111-411`)
   - Routes messages to appropriate handlers
   - Manages dialog state and page transitions
   - Handles keyboard shortcuts and global actions

2. **Page Management** (`tui/tui.go:553-574`)
   - Maintains loaded pages cache
   - Handles page initialization and sizing
   - Prevents navigation during agent operations

3. **Dialog System** (`tui/tui.go:169-321`)
   - Manages modal dialogs overlay
   - Handles permission requests
   - Processes user interactions

---

## Agent Processing Flow

```mermaid
flowchart TD
    A[User Input] --> B[chatPage.SubmitPrompt]
    B --> C[app.AgentCoordinator.Run]
    C --> D[coordinator.Run]
    D --> E[readyWg.Wait]
    E --> F[Get Model Config]
    F --> G[Get Provider Config]
    G --> H[Merge Call Options]
    H --> I[currentAgent.Run]
    I --> J[sessionAgent.Run]
    J --> K[Queue Request]
    K --> L[Process Queue]
    L --> M{Attachments?}
    M -->|Yes| N[Filter by Model Support]
    M -->|No| O[Continue]
    N --> O
    O --> P[Build Fantasy Messages]
    P --> Q[Get Session History]
    Q --> R[Add System Prompt]
    R --> S[Add User Message]
    S --> T[fantasy.Agent.Run]
    T --> U[LLM Provider Call]
    U --> V{Response Type}
    V -->|Text| W[Process Text Response]
    V -->|Tool Call| X[Handle Tool Call]
    V -->|Streaming| Y[Process Streaming]
    
    X --> X1[Parse Tool Call]
    X1 --> X2[Find Tool]
    X2 --> X3[Execute Tool]
    X3 --> X4[Get Tool Result]
    X4 --> X5[Send Result to LLM]
    X5 --> U
    
    W --> W1[Save Message]
    W1 --> W2[Notify TUI]
    W2 --> W3[Display Response]
    
    Y --> Y1[Stream Tokens]
    Y1 --> Y2[Update UI in Real-time]
    Y2 --> Y3[Final Message Save]
    
    J --> J1[Check Rate Limits]
    J1 --> J2{Rate Limited?}
    J2 -->|Yes| J3[Queue Request]
    J2 -->|No| J4[Process Immediately]
    J3 --> L
    J4 --> M
    
    style A fill:#e1f5fe
    style U fill:#fff3e0
    style X fill:#f3e5f5
    style W fill:#e8f5e8
```

### Agent Processing Details

The agent system handles AI interactions:

1. **Request Queuing** (`internal/agent/agent.go`)
   - Manages concurrent requests per session
   - Handles rate limiting and priority
   - Maintains cancellation contexts

2. **Tool Execution** (`internal/agent/tools/`)
   - Dynamic tool discovery and execution
   - Permission validation
   - Result processing and error handling

3. **Response Streaming** (`internal/agent/coordinator.go:110-145`)
   - Real-time response streaming
   - Token usage tracking
   - Error recovery

---

## Tool Execution Flow

```mermaid
flowchart TD
    A[Agent Tool Call] --> B[Parse Tool Request]
    B --> C[Validate Permissions]
    C --> D{Permission Granted?}
    D -->|No| E[Request Permission]
    E --> F[Show Permission Dialog]
    F --> G{User Response}
    G -->|Allow| H[Grant Permission]
    G -->|Deny| I[Deny Request]
    G -->|Allow for Session| J[Grant Persistent]
    H --> K[Execute Tool]
    I --> L[Return Error]
    J --> K
    
    K --> M{Tool Type}
    M -->|bash| N[Execute Shell Command]
    M -->|edit| P[Modify File]
    M -->|view| Q[Read File]
    M -->|glob| R[Find Files]
    M -->|grep| S[Search Content]
    M -->|fetch| T[Web Request]
    M -->|mcp| U[MCP Tool Call]
    M -->|job_*| V[Background Job]
    
    N --> N1[Create Background Shell]
    N1 --> N2[Execute Command]
    N2 --> N3[Monitor Output]
    N3 --> N4[Return Result]
    
    P --> P1[Read File First]
    P1 --> P2[Apply Changes]
    P2 --> P3[Validate Edit]
    P3 --> P4[Save File]
    P4 --> P5[Update History]
    
    Q --> Q1[Check Permissions]
    Q1 --> Q2[Read File Content]
    Q2 --> Q3[Format Output]
    
    R --> R1[Compile Pattern]
    R1 --> R2[Search Filesystem]
    R2 --> R3[Return Matches]
    
    S --> S1[Build Regex Pattern]
    S1 --> S2[Search File Contents]
    S2 --> S3[Return Results]
    
    T --> T1[Validate URL]
    T1 --> T2[Make HTTP Request]
    T2 --> T3[Process Response]
    T3 --> T4[Extract Content]
    
    U --> U1[Get MCP Client]
    U1 --> U2[Call MCP Tool]
    U2 --> U3[Process MCP Response]
    
    V --> V1[Create Background Process]
    V1 --> V2[Store Job ID]
    V2 --> V3[Return Job ID]
    
    N4 --> W[Tool Result]
    P5 --> W
    Q3 --> W
    R3 --> W
    S3 --> W
    T4 --> W
    U3 --> W
    V3 --> W
    L --> X[Tool Error]
    W --> Y[Return to Agent]
    X --> Y
    
    style A fill:#e1f5fe
    style K fill:#e8f5e8
    style W fill:#fff3e0
    style X fill:#ffebee
```

### Tool System Details

The tool system provides file system and development capabilities:

1. **Permission Validation** (`internal/permission/`)
   - Checks tool access permissions
   - Handles user approval workflow
   - Maintains session-specific permissions

2. **Tool Categories** (`internal/agent/tools/`)
   - **File Operations**: `edit.go`, `view.go`, `write.go`, `multiedit.go`
   - **Shell Operations**: `bash.go`, `job_kill.go`, `job_output.go`
   - **Search Operations**: `glob.go`, `grep.go`, `sourcegraph.go`
   - **Network Operations**: `fetch.go`, `agentic_fetch.go`, `download.go`
   - **LSP Operations**: `diagnostics.go`, `references.go`
   - **MCP Tools**: `mcp-tools.go`

3. **Background Processing** (`internal/shell/background.go`)
   - Manages long-running shell commands
   - Provides job control capabilities
   - Handles output streaming

---

## Permission Management Flow

```mermaid
flowchart TD
    A[Tool Execution Request] --> B[permission.Service.Check]
    B --> C{Skip Requests?}
    C -->|Yes| D[Auto-Approve]
    C -->|No| E[Check Permission Cache]
    E --> F{Permission Cached?}
    F -->|Yes| G{Cached Decision}
    F -->|No| H[Create Permission Request]
    
    G -->|Allowed| D
    G -->|Denied| I[Deny Request]
    
    H --> I[Build Permission Object]
    I --> J[publish PermissionRequest]
    J --> K[TUI Receives Event]
    K --> L[Open Permission Dialog]
    L --> M[Display Request Details]
    M --> N[Show User Options]
    
    N --> O{User Choice}
    O -->|Allow| P[Grant Temporary Permission]
    O -->|Allow for Session| Q[Grant Session Permission]
    O -->|Deny| R[Deny Permission]
    
    P --> S[permission.Service.Grant]
    Q --> T[permission.Service.GrantPersistent]
    R --> U[permission.Service.Deny]
    
    S --> V[Execute Tool]
    T --> W[Cache Permission for Session]
    W --> X[Execute Tool]
    U --> Y[Return Permission Error]
    
    V --> Z[Tool Result]
    X --> Z
    Y --> AA[Tool Error]
    
    style A fill:#e1f5fe
    style D fill:#e8f5e8
    style L fill:#fff3e0
    style Z fill:#fff3e0
    style Y fill:#ffebee
```

### Permission System Details

The permission system ensures secure tool execution:

1. **Permission Types** (`internal/permission/permission.go`)
   - **Temporary**: Single-use permission
   - **Session**: Permission for current session
   - **Persistent**: Saved permission for future sessions

2. **Permission Dialog** (`internal/tui/components/dialogs/permissions/`)
   - Shows tool request details
   - Displays file changes (diff mode)
   - Provides allow/deny options

3. **Auto-Approval Modes**
   - **YOLO Mode**: `-y` flag auto-approves all requests
   - **Session Auto-approval**: Non-interactive mode

---

## Message Flow Architecture

```mermaid
flowchart TD
    A[User Input] --> B[chatPage.editor]
    B --> C[Submit Message]
    C --> D[message.Service.Create]
    D --> E[Save to Database]
    E --> F[publish Message Created]
    F --> G[TUI Receives Event]
    G --> H[Update Message List]
    
    C --> I[AgentCoordinator.Run]
    I --> J[AI Processing]
    J --> K[AI Response]
    K --> L[message.Service.Create]
    L --> M[Save Assistant Message]
    M --> N[publish Message Created]
    N --> O[TUI Receives Event]
    O --> P[Update Message List]
    
    P --> Q{Streaming Response?}
    Q -->|Yes| R[Stream Updates]
    Q -->|No| S[Final Update]
    
    R --> R1[Update Message Content]
    R1 --> R2[publish Message Updated]
    R2 --> R3[TUI Updates in Real-time]
    
    J --> J1{Tool Calls?}
    J1 -->|Yes| J2[Tool Execution]
    J2 --> J3[Tool Results]
    J3 --> J4[Add to Message]
    J4 --> K
    
    H --> H1[Scroll to Bottom]
    H1 --> H2[Update UI]
    
    R3 --> R4[Scroll to Bottom]
    R4 --> R5[Update UI]
    
    S --> S1[Scroll to Bottom]
    S1 --> S2[Complete Update]
    
    style A fill:#e1f5fe
    style J fill:#fff3e0
    style K fill:#e8f5e8
    style R3 fill:#e8f5e8
```

### Message System Details

The message system handles all conversation data:

1. **Message Types** (`internal/message/message.go`)
   - **User**: User input messages
   - **Assistant**: AI responses
   - **System**: System notifications
   - **Tool**: Tool call and results

2. **Message Storage** (`internal/db/messages.sql.go`)
   - SQLite database storage
   - Full-text search capabilities
   - Attachment support

3. **Event Publishing** (`internal/pubsub/`)
   - Real-time message updates
   - TUI synchronization
   - Component communication

---

## LSP Integration Flow

```mermaid
flowchart TD
    A[App Startup] --> B[app.initLSPClients]
    B --> C[Read LSP Config]
    C --> D{LSP Servers Configured?}
    D -->|No| E[Skip LSP Setup]
    D -->|Yes| F[Create LSP Clients]
    F --> G[For Each LSP Server]
    G --> H[lsp.NewClient]
    H --> I[Start LSP Server Process]
    I --> J[Initialize LSP Protocol]
    J --> K[Store in csync.Map]
    K --> L{More Servers?}
    L -->|Yes| G
    L -->|No| M[LSP Ready]
    
    E --> N[Continue Startup]
    M --> N
    
    O[Tool Request] --> P{LSP Tool?}
    P -->|No| Q[Other Tool Handler]
    P -->|Yes| R[Get LSP Client]
    R --> S[Extract File Path]
    S --> T[Find Matching Client]
    T --> U{Client Found?}
    U -->|No| V[Return Error]
    U -->|Yes| W[Call LSP Method]
    
    W --> X{LSP Method}
    X -->|Diagnostics| Y[lsp.GetDiagnostics]
    X -->|References| Z[lsp.FindReferences]
    X -->|Symbols| AA[lsp.DocumentSymbols]
    X -->|Definition| BB[lsp.GotoDefinition]
    
    Y --> CC[Parse Diagnostics]
    CC --> DD[Format Results]
    DD --> EE[Return to Tool]
    
    Z --> FF[Search References]
    FF --> GG[Format Results]
    GG --> EE
    
    AA --> HH[Extract Symbols]
    HH --> II[Format Results]
    II --> EE
    
    BB --> JJ[Find Definition]
    JJ --> KK[Format Results]
    KK --> EE
    
    EE --> LL[Tool Result]
    LL --> MM[Return to Agent]
    
    style A fill:#e1f5fe
    style H fill:#fff3e0
    style W fill:#e8f5e8
    style LL fill:#fff3e0
```

### LSP System Details

The LSP integration provides IDE-like capabilities:

1. **LSP Client Management** (`internal/lsp/client.go`)
   - Process lifecycle management
   - Protocol communication
   - Error handling and reconnection

2. **LSP Tools** (`internal/agent/tools/`)
   - **Diagnostics**: `diagnostics.go` - error/warning highlighting
   - **References**: `references.go` - find usages and go-to-definition

3. **Language Detection** (`internal/lsp/language.go`)
   - File extension mapping
   - Dynamic server selection
   - Configuration inheritance

---

## Session Management Flow

```mermaid
flowchart TD
    A[App Start] --> B[Load Existing Sessions]
    B --> C[session.Service.List]
    C --> D[Query Database]
    D --> E[Return Session List]
    E --> F[TUI Displays Sessions]
    
    G[User Action] --> H{Action Type}
    H -->|New Session| I[Create New Session]
    H -->|Switch Session| J[Switch Session]
    H -->|Delete Session| K[Delete Session]
    
    I --> I1[Generate Session ID]
    I1 --> I2[session.Service.Create]
    I2 --> I3[Save to Database]
    I3 --> I4[publish Session Created]
    I4 --> I5[Switch to New Session]
    
    J --> J1[Select Session ID]
    J1 --> J2[Load Session Messages]
    J2 --> J3[Update TUI State]
    J3 --> J4[Clear Agent Queue]
    J4 --> J5[Update Selected Session]
    
    K --> K1[Confirm Delete]
    K1 --> K2[session.Service.Delete]
    K2 --> K3[Cascade Delete Messages]
    K3 --> K4[publish Session Deleted]
    K4 --> K5[Remove from UI]
    K5 --> K6{Current Session?}
    K6 -->|Yes| K7[Switch to Default]
    K6 -->|No| K8[Continue]
    
    L[Message Created] --> M[Update Session Timestamp]
    M --> N[Mark Session Active]
    N --> O[publish Session Updated]
    O --> P[TUI Updates Session List]
    
    Q[Session Summary] --> R[AgentCoordinator.Summarize]
    R --> S[Create Summary Request]
    S --> T[Process with AI]
    T --> U[Generate Summary]
    U --> V[Create Summary Message]
    V --> W[Link to Original]
    W --> X[Mark as Summary]
    X --> Y[Update Session Display]
    
    style A fill:#e1f5fe
    style I fill:#e8f5e8
    style J fill:#fff3e0
    style K fill:#ffebee
    style R fill:#fff3e0
```

### Session System Details

The session system manages conversation contexts:

1. **Session Lifecycle** (`internal/session/session.go`)
   - Creation, deletion, and switching
   - Message association
   - Metadata management

2. **Session Features**
   - **Auto-save**: Messages automatically saved
   - **Summarization**: Long conversation compression
   - **Persistence**: Survives application restarts

3. **Database Operations** (`internal/db/sessions.sql.go`)
   - CRUD operations
   - Message relationship management
   - Full-text search indexing

---

## Summary

The Crush application follows a clean, event-driven architecture with clear separation of concerns:

### Key Architectural Patterns

1. **Event-Driven Communication**
   - Pub/sub system for loose coupling
   - Real-time UI updates
   - Async message processing

2. **Service Layer Pattern**
   - Clear service boundaries
   - Dependency injection
   - Testable components

3. **Tool-Based Agent System**
   - Extensible tool ecosystem
   - Permission-gated execution
   - Background processing support

4. **Reactive UI Architecture**
   - Elm-style update loop
   - State management
   - Component composition

### Critical Flow Integration Points

1. **User Input → Agent Processing**
   - TUI captures and validates input
   - Agent coordinator manages AI interaction
   - Tools provide execution capabilities

2. **Permission System Integration**
   - All tool operations pass through permissions
   - User approval workflow
   - Session-based permission caching

3. **LSP Integration**
   - Language-specific server management
   - IDE-like features in terminal
   - Tool integration for code intelligence

This architecture enables a powerful, extensible AI assistant that operates entirely within the terminal while maintaining security and performance standards.