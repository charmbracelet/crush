# CheckpointSystem 檢查點系統

## 概述

CheckpointSystem 是 CrushCL 多代理系統中的檢查點和恢復元件，負責保存系統狀態快照、支援故障恢復和任務繼續執行。

## 設計目標

| 目標 | 說明 |
|------|------|
| **狀態保存** | 定期保存代理和任務的狀態 |
| **快速恢復** | 支援從檢查點快速恢復 |
| **增量保存** | 只保存自上次檢查點以來的變更 |
| **一致性** | 確保檢查點的一致性 |
| **壓縮存儲** | 壓縮檢查點以減少存儲空間 |

## 核心類型

### CheckpointSystem 結構

```go
type CheckpointSystem struct {
    mu           sync.RWMutex
    config       CheckpointConfig
    checkpoints  map[string]*Checkpoint
    snapshotter  *Snapshotter
    storage      CheckpointStorage
    recovery     *RecoveryManager
    listeners    []CheckpointListener
}

type Checkpoint struct {
    ID            string
    Type          CheckpointType
    TargetID      string       // 任務ID或代理ID
    Data          []byte       // 序列化的狀態數據
    Metadata      CheckpointMetadata
    CreatedAt     time.Time
    ExpiresAt     time.Time    // 過期時間
    Compression   CompressionType
}

type CheckpointMetadata struct {
    Version       int
    SequenceNum   int64
    Checksum      string
    Size          int64
    ParentID      string        // 父檢查點 ID（用於增量）
    StateBefore   string        // 狀態機狀態
    StateAfter    string
    AgentID       string
    TaskID        string
    Tags          map[string]string
}

type CheckpointType int

const (
    CheckpointTypeAgent CheckpointType = iota
    CheckpointTypeTask
    CheckpointTypeSession
    CheckpointTypeSystem
)
```

## 配置參數

```go
type CheckpointConfig struct {
    Enabled           bool
    Interval          time.Duration        // 檢查點間隔
    MaxCheckpoints    int                 // 最大檢查點數量
    MaxAge            time.Duration       // 檢查點最大保存時間
    StoragePath       string              // 存儲路徑
    CompressionEnabled bool                // 是否壓縮
    IncrementalEnabled bool               // 是否啟用增量
    SyncEnabled       bool                // 是否同步寫入
    RetryAttempts     int                 // 重試次數
    RetryDelay        time.Duration       // 重試延遲
}

type CompressionType int

const (
    CompressionNone CompressionType = iota
    CompressionGZIP
    CompressionLZ4
    CompressionZSTD
)
```

## 介面定義

```go
type CheckpointSystemInterface interface {
    // 創建檢查點
    CreateCheckpoint(targetID string, data interface{}, cpType CheckpointType) (*Checkpoint, error)
    
    // 獲取檢查點
    GetCheckpoint(checkpointID string) (*Checkpoint, error)
    
    // 獲取最新的檢查點
    GetLatestCheckpoint(targetID string) (*Checkpoint, error)
    
    // 刪除檢查點
    DeleteCheckpoint(checkpointID string) error
    
    // 恢復到檢查點
    Restore(checkpointID string) error
    
    // 列出目標的所有檢查點
    ListCheckpoints(targetID string) ([]*Checkpoint, error)
    
    // 清理過期檢查點
    CleanupExpired() (int, error)
}

type CheckpointListener interface {
    OnCheckpointCreated(checkpoint *Checkpoint)
    OnCheckpointRestored(checkpoint *Checkpoint)
    OnCheckpointDeleted(checkpointID string)
}
```

## 核心功能

### 1. 檢查點創建

```go
func (cs *CheckpointSystem) CreateCheckpoint(targetID string, data interface{}, cpType CheckpointType) (*Checkpoint, error) {
    cs.mu.Lock()
    defer cs.mu.Unlock()
    
    // 序列化數據
    serialized, err := cs.serialize(data)
    if err != nil {
        return nil, fmt.Errorf("failed to serialize data: %w", err)
    }
    
    // 獲取元數據
    metadata := cs.buildMetadata(targetID, cpType)
    
    // 壓縮（如果啟用）
    if cs.config.CompressionEnabled {
        compressed, err := cs.compress(serialized, cs.config.CompressionType)
        if err != nil {
            return nil, fmt.Errorf("failed to compress: %w", err)
        }
        serialized = compressed
        metadata.Compression = cs.config.CompressionType
    }
    
    // 計算校驗和
    metadata.Checksum = cs.calculateChecksum(serialized)
    metadata.Size = int64(len(serialized))
    
    // 獲取父檢查點（增量）
    var parentID string
    if cs.config.IncrementalEnabled {
        if latest := cs.getLatestUnlocked(targetID); latest != nil {
            parentID = latest.ID
        }
    }
    metadata.ParentID = parentID
    
    // 創建檢查點
    checkpoint := &Checkpoint{
        ID:        cs.generateID(),
        Type:      cpType,
        TargetID:  targetID,
        Data:      serialized,
        Metadata:  metadata,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(cs.config.MaxAge),
    }
    
    // 保存到存儲
    if err := cs.storage.Save(checkpoint); err != nil {
        return nil, fmt.Errorf("failed to save checkpoint: %w", err)
    }
    
    // 添加到內存映射
    cs.checkpoints[checkpoint.ID] = checkpoint
    
    // 觸發事件
    cs.notifyListeners(checkpoint)
    
    // 檢查是否需要清理
    go cs.maybeCleanup()
    
    return checkpoint, nil
}

func (cs *CheckpointSystem) serialize(data interface{}) ([]byte, error) {
    return json.Marshal(data)
}

func (cs *CheckpointSystem) compress(data []byte, compression CompressionType) ([]byte, error) {
    switch compression {
    case CompressionGZIP:
        return cs.compressGZIP(data)
    case CompressionLZ4:
        return cs.compressLZ4(data)
    case CompressionZSTD:
        return cs.compressZSTD(data)
    default:
        return data, nil
    }
}
```

### 2. 檢查點恢復

```go
func (cs *CheckpointSystem) Restore(checkpointID string) error {
    cs.mu.Lock()
    checkpoint, ok := cs.checkpoints[checkpointID]
    if !ok {
        cs.mu.Unlock()
        return ErrCheckpointNotFound
    }
    cs.mu.Unlock()
    
    // 獲取完整的增量鏈
    chain, err := cs.getIncrementalChain(checkpoint)
    if err != nil {
        return fmt.Errorf("failed to get incremental chain: %w", err)
    }
    
    // 合併增量數據
    mergedData, err := cs.mergeIncrementalData(chain)
    if err != nil {
        return fmt.Errorf("failed to merge incremental data: %w", err)
    }
    
    // 解壓縮
    if checkpoint.Metadata.Compression != CompressionNone {
        mergedData, err = cs.decompress(mergedData, checkpoint.Metadata.Compression)
        if err != nil {
            return fmt.Errorf("failed to decompress: %w", err)
        }
    }
    
    // 驗證校驗和
    if !cs.verifyChecksum(mergedData, checkpoint.Metadata.Checksum) {
        return ErrChecksumMismatch
    }
    
    // 觸發恢復事件
    cs.notifyRestoreListeners(checkpoint)
    
    return nil
}

func (cs *CheckpointSystem) getIncrementalChain(checkpoint *Checkpoint) ([]*Checkpoint, error) {
    var chain []*Checkpoint
    
    current := checkpoint
    for current.Metadata.ParentID != "" {
        chain = append(chain, current)
        
        parent, err := cs.GetCheckpoint(current.Metadata.ParentID)
        if err != nil {
            return nil, err
        }
        current = parent
    }
    chain = append(chain, current)
    
    // 反轉鏈表（從根到葉）
    for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
        chain[i], chain[j] = chain[j], chain[i]
    }
    
    return chain, nil
}

func (cs *CheckpointSystem) mergeIncrementalData(chain []*Checkpoint) ([]byte, error) {
    // 從根節點開始應用增量
    var baseData []byte
    
    for _, cp := range chain {
        if cp.Metadata.ParentID == "" {
            // 根節點，直接使用
            baseData = cp.Data
        } else {
            // 增量節點，應用補丁
            var err error
            baseData, err = cs.applyPatch(baseData, cp.Data)
            if err != nil {
                return nil, err
            }
        }
    }
    
    return baseData, nil
}
```

### 3. 增量檢查點

```go
type IncrementalSnapshotter struct {
    baseSnapshot []byte
    patches     [][]byte
}

func (cs *CheckpointSystem) createIncrementalCheckpoint(targetID string, newState []byte) (*Checkpoint, error) {
    latest := cs.getLatestUnlocked(targetID)
    
    if latest == nil {
        // 沒有父檢查點，創建完整快照
        return cs.createFullCheckpoint(targetID, newState)
    }
    
    // 計算差異補丁
    var parentData []byte
    if latest.Metadata.Compression != CompressionNone {
        var err error
        parentData, err = cs.decompress(latest.Data, latest.Metadata.Compression)
        if err != nil {
            return nil, err
        }
    } else {
        parentData = latest.Data
    }
    
    // 生成補丁
    patch, err := cs.diff(parentData, newState)
    if err != nil {
        // 如果差異失敗，創建完整快照
        return cs.createFullCheckpoint(targetID, newState)
    }
    
    // 創建增量檢查點
    return &Checkpoint{
        ID:       cs.generateID(),
        Type:     CheckpointTypeTask,
        TargetID: targetID,
        Data:     patch,
        Metadata: CheckpointMetadata{
            Version:     1,
            SequenceNum: latest.Metadata.SequenceNum + 1,
            ParentID:    latest.ID,
            Compression: CompressionLZ4, // 補丁使用 LZ4 壓縮
        },
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(cs.config.MaxAge),
    }, nil
}
```

### 4. 檢查點清理

```go
func (cs *CheckpointSystem) CleanupExpired() (int, error) {
    cs.mu.Lock()
    defer cs.mu.Unlock()
    
    now := time.Now()
    var toDelete []*Checkpoint
    
    // 收集過期的檢查點
    for _, cp := range cs.checkpoints {
        if now.After(cp.ExpiresAt) {
            toDelete = append(toDelete, cp)
        }
    }
    
    // 限制最大刪除數量
    if len(toDelete) > cs.config.MaxCheckpoints/2 {
        toDelete = toDelete[:cs.config.MaxCheckpoints/2]
    }
    
    // 刪除
    deleted := 0
    for _, cp := range toDelete {
        if err := cs.storage.Delete(cp.ID); err != nil {
            continue
        }
        delete(cs.checkpoints, cp.ID)
        deleted++
    }
    
    // 如果仍然超過最大數量，刪除最舊的
    if len(cs.checkpoints) > cs.config.MaxCheckpoints {
        sorted := cs.sortByTime()
        for i := cs.config.MaxCheckpoints; i < len(sorted); i++ {
            if err := cs.storage.Delete(sorted[i].ID); err != nil {
                continue
            }
            delete(cs.checkpoints, sorted[i].ID)
            deleted++
        }
    }
    
    return deleted, nil
}

func (cs *CheckpointSystem) sortByTime() []*Checkpoint {
    checkpoints := make([]*Checkpoint, 0, len(cs.checkpoints))
    for _, cp := range cs.checkpoints {
        checkpoints = append(checkpoints, cp)
    }
    
    sort.Slice(checkpoints, func(i, j int) bool {
        return checkpoints[i].CreatedAt.Before(checkpoints[j].CreatedAt)
    })
    
    return checkpoints
}
```

### 5. 快照管理

```go
type Snapshotter struct {
    mu       sync.Mutex
    snapshots map[string]*Snapshot
    config   SnapshotConfig
}

type Snapshot struct {
    ID        string
    TargetID  string
    State     interface{}
    Timestamp time.Time
    Size      int64
}

func (s *Snapshotter) CreateSnapshot(targetID string, state interface{}) (*Snapshot, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    serialized, err := json.Marshal(state)
    if err != nil {
        return nil, err
    }
    
    snapshot := &Snapshot{
        ID:        generateID(),
        TargetID:  targetID,
        State:     state,
        Timestamp: time.Now(),
        Size:      int64(len(serialized)),
    }
    
    s.snapshots[targetID] = snapshot
    return snapshot, nil
}

func (s *Snapshotter) GetSnapshot(targetID string) (*Snapshot, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    snapshot, ok := s.snapshots[targetID]
    if !ok {
        return nil, ErrSnapshotNotFound
    }
    
    return snapshot, nil
}
```

## 存儲介面

```go
type CheckpointStorage interface {
    Save(checkpoint *Checkpoint) error
    Load(checkpointID string) (*Checkpoint, error)
    Delete(checkpointID string) error
    List(targetID string) ([]*Checkpoint, error)
    Exists(checkpointID string) (bool, error)
}

type FileSystemStorage struct {
    basePath string
    mu       sync.Mutex
}

func (f *FileSystemStorage) Save(checkpoint *Checkpoint) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    
    path := f.getPath(checkpoint.ID)
    
    file, err := os.Create(path)
    if err != nil {
        return err
    }
    defer file.Close()
    
    encoder := json.NewEncoder(file)
    return encoder.Encode(checkpoint)
}

func (f *FileSystemStorage) Load(checkpointID string) (*Checkpoint, error) {
    path := f.getPath(checkpointID)
    
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var checkpoint Checkpoint
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&checkpoint)
    return &checkpoint, err
}
```

## 使用範例

### 基本用法

```go
// 創建檢查點系統
checkpointSystem := NewCheckpointSystem(CheckpointConfig{
    Enabled:           true,
    Interval:          1 * time.Minute,
    MaxCheckpoints:    100,
    MaxAge:            24 * time.Hour,
    StoragePath:       "./checkpoints",
    CompressionEnabled: true,
    IncrementalEnabled: true,
})

// 創建檢查點
type TaskState struct {
    Status    string
    Progress  int
    Result    interface{}
}

state := &TaskState{
    Status:   "running",
    Progress: 50,
    Result:   nil,
}

cp, err := checkpointSystem.CreateCheckpoint("task-123", state, CheckpointTypeTask)

// 恢復檢查點
err = checkpointSystem.Restore(cp.ID)

// 列出檢查點
checkpoints, _ := checkpointSystem.ListCheckpoints("task-123")
fmt.Printf("找到 %d 個檢查點\n", len(checkpoints))
```

### 與代理整合

```go
type AgentWithCheckpoint struct {
    agent    Agent
    checkpoint *CheckpointSystem
}

func (a *AgentWithCheckpoint) SaveState() error {
    state := a.agent.GetState()
    _, err := a.checkpoint.CreateCheckpoint(
        a.agent.ID(),
        state,
        CheckpointTypeAgent,
    )
    return err
}

func (a *AgentWithCheckpoint) Restore() error {
    latest, err := a.checkpoint.GetLatestCheckpoint(a.agent.ID())
    if err != nil {
        return err
    }
    return a.checkpoint.Restore(latest.ID)
}

func (a *AgentWithCheckpoint) PeriodicCheckpoint(ctx context.Context) {
    ticker := time.NewTicker(a.checkpoint.config.Interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := a.SaveState(); err != nil {
                log.Printf("Checkpoint failed: %v", err)
            }
        }
    }
}
```

## 錯誤處理

```go
var (
    ErrCheckpointNotFound = errors.New("checkpoint not found")
    ErrChecksumMismatch   = errors.New("checksum mismatch")
    ErrStorageFull       = errors.New("checkpoint storage full")
    ErrInvalidCheckpoint  = errors.New("invalid checkpoint data")
    ErrSnapshotNotFound   = errors.New("snapshot not found")
)
```

## 與其他元件的整合

| 元件 | 整合方式 |
|------|---------|
| SwarmExt | 任務狀態保存和恢復 |
| Guardian | 故障後代理狀態恢復 |
| StateMachine | 狀態機快照 |
| Session | 會話狀態持久化 |

## 下一步

- [ ] 實現檢查點存儲介面
- [ ] 添加分散式存儲支援
- [ ] 實現增量快照優化
- [ ] 添加檢查點視覺化工具
