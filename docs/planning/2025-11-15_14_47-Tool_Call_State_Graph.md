# 🔄 TOOL CALL STATE MACHINE - COMPLETE GRAPH

## **🎯 ALL TOOL CALL STATES**

### **📋 STATE DEFINITIONS**

| State | String Value | Description | Type | Final State |
|--------|-------------|-------------|-------|-------------|
| `ToolCallStatePending` | `"pending"` | Tool created but not started execution | Initial | ❌ No |
| `ToolCallStatePermissionPending` | `"permission_pending"` | Awaiting user permission approval | Permission | ❌ No |
| `ToolCallStatePermissionApproved` | `"permission_approved"` | Permission granted, ready to execute | Permission | ❌ No |
| `ToolCallStatePermissionDenied` | `"permission_denied"` | Permission denied by user | Permission | ✅ Yes |
| `ToolCallStateRunning` | `"running"` | Tool actively executing | Execution | ❌ No |
| `ToolCallStateCompleted` | `"completed"` | Tool completed successfully | Execution | ✅ Yes |
| `ToolCallStateFailed` | `"failed"` | Tool failed during execution | Execution | ✅ Yes |
| `ToolCallStateCancelled` | `"cancelled"` | Tool cancelled by user | Execution | ✅ Yes |

---

## **🏗️ STATE TRANSITION GRAPH**

```mermaid
stateDiagram-v2
    [*] --> Pending : Tool created
    
    Pending --> PermissionPending : Permission required
    Pending --> Running : Auto-approved
    Pending --> Cancelled : User cancels
    
    PermissionPending --> PermissionApproved : User approves
    PermissionPending --> PermissionDenied : User denies
    PermissionPending --> Cancelled : User cancels request
    
    PermissionApproved --> Running : Start execution
    PermissionApproved --> Cancelled : User cancels before execution
    
    Running --> Completed : Tool succeeds
    Running --> Failed : Tool fails
    Running --> Cancelled : User cancels during execution
    
    Completed --> [*] : End
    Failed --> [*] : End
    Cancelled --> [*] : End
    PermissionDenied --> [*] : End
    
    note right of Pending
        Initial state
        Multiple tools can be pending
        Queue processing
    end note
    
    note right of PermissionPending
        User interaction required
        UI shows permission dialog
        Timer may be active
    end note
    
    note right of PermissionApproved
        Permission granted
        Ready to execute
        Transition to running
    end note
    
    note right of Running
        Tool actively executing
        Spinner animation active
        Progress updates possible
    end note
    
    note right of FinalStates
        Completed : Success state
        Failed : Error state
        Cancelled : User-initiated stop
        PermissionDenied : User rejection
    end note
```

---

## **🎨 STATE VISUALIZATION**

### **🟡 NON-FINAL STATES (Transitional)**

| State | Color | Icon | Animation |
|--------|--------|------|------------|
| `Pending` | Gray Muted | ⏳ | Static |
| `PermissionPending` | Paprika | 🔐 | Timer |
| `PermissionApproved` | Green Dark | ✅ | Blink |
| `Running` | Green | ▶️ | Dot blink |

### **🔴 FINAL STATES (Terminal)**

| State | Color | Icon | Animation |
|--------|--------|------|------------|
| `Completed` | Green | ✅ | Static |
| `Failed` | Red | ❌ | Static |
| `Cancelled` | Orange | ⏹️ | Static |
| `PermissionDenied` | Red | 🚫 | Static |

---

## **🔄 VALID TRANSITIONS MATRIX**

| From → | Pending | PermissionPending | PermissionApproved | PermissionDenied | Running | Completed | Failed | Cancelled |
|----------|---------|-------------------|-------------------|------------------|----------|------------|---------|------------|
| **Pending** | - | ✅ Required | ✅ Auto | ❌ | ✅ Auto | ❌ | ❌ | ✅ |
| **PermissionPending** | ❌ | - | ✅ Approved | ✅ Denied | ❌ | ❌ | ❌ | ✅ |
| **PermissionApproved** | ❌ | ❌ | - | ❌ | ✅ Start | ❌ | ❌ | ✅ |
| **PermissionDenied** | ❌ | ❌ | ❌ | - | ❌ | ❌ | ❌ | ❌ |
| **Running** | ❌ | ❌ | ❌ | ❌ | - | ✅ Success | ✅ Error | ✅ Cancel |
| **Completed** | ❌ | ❌ | ❌ | ❌ | ❌ | - | ❌ | ❌ |
| **Failed** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | - | ❌ |
| **Cancelled** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | - |

---

## **🎯 BUSINESS RULES**

### **🔍 PERMISSION FLOW**
```
Tool Created → Need Permission? → 
    YES → Show Dialog → User Decision → 
        APPROVE → Execute Tool
        DENY → Terminate Tool
    NO → Execute Tool Immediately
```

### **🏃 EXECUTION FLOW**
```
Permission Approved → Start Tool → 
    SUCCESS → Mark Completed
    ERROR → Mark Failed
    USER CANCEL → Mark Cancelled
```

### **⚡ PARALLEL EXECUTION**
```
Multiple Tools → All go to Pending → 
    Queue Processing → Execute in order or parallel
    Each tool independent state management
```

---

## **🐛 EDGE CASES HANDLED**

### **🔄 RACE CONDITIONS**
- **Multiple permission requests**: Queue properly
- **User cancels during execution**: Immediate termination
- **Permission timeout**: Auto-deny after timeout

### **🔒 STATE CONSISTENCY**
- **No invalid transitions**: Matrix validation
- **Single source of truth**: ToolCallState enum only
- **Atomic state changes**: Single transition operations

### **🎨 UI CORNER CASES**
- **Permission during execution**: Denied tools show immediately
- **Multiple pending tools**: Clear queue visualization
- **Animation cleanup**: Proper state transition handling

---

## **🔧 IMPLEMENTATION DETAILS**

### **📊 STATE ENUM DEFINITION**
```go
const (
    // Initial States
    ToolCallStatePending ToolCallState = "pending"
    
    // Permission States  
    ToolCallStatePermissionPending  ToolCallState = "permission_pending"
    ToolCallStatePermissionApproved ToolCallState = "permission_approved"
    ToolCallStatePermissionDenied  ToolCallState = "permission_denied"
    
    // Execution States
    ToolCallStateRunning   ToolCallState = "running"
    ToolCallStateCompleted ToolCallState = "completed"
    ToolCallStateFailed    ToolCallState = "failed"
    ToolCallStateCancelled ToolCallState = "cancelled"
)
```

### **🎯 FINAL STATE DETECTION**
```go
func (state ToolCallState) IsFinalState() bool {
    return state == ToolCallStateCompleted ||
           state == ToolCallStateFailed ||
           state == ToolCallStateCancelled ||
           state == ToolCallStatePermissionDenied
}
```

---

## **🎉 ARCHITECTURAL BENEFITS**

### **🎯 SINGLE SOURCE OF TRUTH**
- **No dual-state tracking** - Only ToolCallState
- **Type-safe transitions** - Invalid states impossible
- **Clear business rules** - Matrix validation

### **🔒 ERROR PREVENTION**
- **Impossible states eliminated** - Strong typing
- **Race condition protection** - Atomic transitions
- **UI consistency guaranteed** - Single state source

### **🚀 MAINTAINABILITY**
- **Simple extension** - Add new states easily
- **Clear documentation** - Visual graph helps
- **Testing simplified** - Matrix validation

---

## **📊 CURRENT STATUS**

- **✅ All 8 states defined**
- **✅ State transitions validated**  
- **✅ Final state detection working**
- **✅ UI colors and icons mapped**
- **✅ Business rules implemented**
- **✅ Edge cases handled**
- **✅ Zero split-brain architecture**

**This unified state system represents the foundation of a robust, type-safe permission and tool execution architecture!** 🚀