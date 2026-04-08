# ConsensusProtocol 共識協議

## 概述

ConsensusProtocol 是 CrushCL 多代理系統中的共識協議元件，負責在多個代理之間達成一致決策，確保系統行為的協同性。

## 設計目標

| 目標 | 說明 |
|------|------|
| **一致性保證** | 確保所有代理對最終決策達成一致 |
| **容錯性** | 部分代理失敗不影響整體共識 |
| **公平性** | 每個代理都有平等的投票權 |
| **效率** | 在合理的時間內達成共識 |
| **可擴展性** | 支持動態添加/移除代理 |

## 共識協議類型

### 1. Raft 風格共識

```go
type RaftConsensus struct {
    mu          sync.RWMutex
    state       RaftState
    currentTerm int
    votedFor    string
    log         []LogEntry
    commitIndex int
    lastApplied int
    
    // 節點資訊
    nodeID      string
    peers       map[string]*Peer
    
    // 通道
    proposeCh   chan *Proposal
    commitCh    chan *Commit
    applyCh     chan *Apply
    
    // 配置
    config      RaftConfig
}

type RaftState int

const (
    RaftStateFollower RaftState = iota
    RaftStateCandidate
    RaftStateLeader
)

type LogEntry struct {
    Term      int
    Index     int
    Command   interface{}
    Data      []byte
}

type Peer struct {
    ID         string
    Address    string
    LastContact time.Time
    MatchIndex int
    NextIndex  int
}
```

### 2. 簡單多數投票

```go
type MajorityConsensus struct {
    mu          sync.RWMutex
    voters      map[string]bool
    votes       map[string]string // proposalID -> voterID
    thresholds  int
    config      ConsensusConfig
}

type Proposal struct {
    ID        string
    Value     interface{}
    Proposer  string
    Votes     map[string]bool
    CreatedAt time.Time
    ExpiresAt time.Time
}
```

## 配置參數

```go
type ConsensusConfig struct {
    ProtocolType     ProtocolType      // 協議類型
    NodeID           string            // 本節點 ID
    Peers            []string         // 對等節點列表
    Timeout          time.Duration    // 投票超時
    HeartbeatInterval time.Duration   // 心跳間隔
    ElectionTimeout   time.Duration   // 選舉超時
    QuorumSize       int              // 法定人數
    RetryAttempts    int              // 重試次數
    RetryDelay       time.Duration    // 重試延遲
}

type ProtocolType int

const (
    ProtocolTypeRaft ProtocolType = iota
    ProtocolTypeMajority
    ProtocolTypeTwoPhase
    ProtocolTypePaxos
)
```

## 介面定義

```go
type ConsensusProtocol interface {
    // 提交提案
    Propose(ctx context.Context, value interface{}) (*Proposal, error)
    
    // 投票
    Vote(ctx context.Context, proposalID string) (bool, error)
    
    // 檢查是否達成共識
    IsConsensus(proposalID string) (bool, error)
    
    // 獲取共識值
    GetConsensusValue(proposalID string) (interface{}, error)
    
    // 添加觀察者
    AddObserver(observer ConsensusObserver)
    
    // 獲取當前領導者
    GetLeader() (string, error)
    
    // 獲取當前狀態
    GetState() (ConsensusState, error)
}

type ConsensusObserver interface {
    OnConsensusReached(proposalID string, value interface{})
    OnProposalRejected(proposalID string, reason string)
    OnLeaderChanged(oldLeader, newLeader string)
    OnStateChanged(oldState, newState ConsensusState)
}

type ConsensusState struct {
    Term        int
    State       string
    Leader      string
    CommitIndex int
    LastLogIndex int
    LastLogTerm  int
}
```

## 核心功能

### 1. 提案處理

```go
func (rc *RaftConsensus) Propose(ctx context.Context, value interface{}) (*Proposal, error) {
    rc.mu.Lock()
    
    // 只有 leader 才能處理提案
    if rc.state != RaftStateLeader {
        if rc.state == RaftStateCandidate {
            // 等待選舉完成
            rc.mu.Unlock()
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(rc.config.ElectionTimeout):
                rc.mu.Lock()
            }
        } else {
            leader, _ := rc.GetLeader()
            rc.mu.Unlock()
            return nil, fmt.Errorf("not leader, current leader: %s", leader)
        }
    }
    
    // 創建日誌條目
    entry := LogEntry{
        Term:    rc.currentTerm,
        Index:   len(rc.log) + 1,
        Command: value,
    }
    rc.log = append(rc.log, entry)
    proposalID := fmt.Sprintf("%d-%d", entry.Term, entry.Index)
    
    proposal := &Proposal{
        ID:        proposalID,
        Value:     value,
        Proposer:  rc.nodeID,
        Votes:     map[string]bool{rc.nodeID: true},
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(rc.config.Timeout),
    }
    
    rc.mu.Unlock()
    
    // 發送給所有追隨者
    go rc.replicateToFollowers(entry)
    
    return proposal, nil
}

func (rc *RaftConsensus) replicateToFollowers(entry LogEntry) {
    rc.mu.RLock()
    peers := make([]*Peer, 0, len(rc.peers))
    for _, peer := range rc.peers {
        peers = append(peers, peer)
    }
    rc.mu.RUnlock()
    
    var wg sync.WaitGroup
    for _, peer := range peers {
        wg.Add(1)
        go func(p *Peer) {
            defer wg.Done()
            rc.replicateToPeer(p, entry)
        }(peer)
    }
    wg.Wait()
}
```

### 2. 投票流程

```go
func (rc *RaftConsensus) Vote(ctx context.Context, proposalID string) (bool, error) {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
    // 解析提案
    var term, index int
    fmt.Sscanf(proposalID, "%d-%d", &term, &index)
    
    // 檢查候選人的日誌是否至少和我的一樣新
    if len(rc.log) < index {
        return false, nil
    }
    
    lastLogTerm := 0
    if len(rc.log) > 0 {
        lastLogTerm = rc.log[len(rc.log)-1].Term
    }
    
    // 日誌完整性檢查
    if term < lastLogTerm {
        return false, nil
    }
    
    // 如果 term 更大，重置投票
    if term > rc.currentTerm {
        rc.currentTerm = term
        rc.votedFor = ""
    }
    
    // 檢查是否已經投票給別人
    if rc.votedFor != "" && rc.votedFor != proposalID {
        return false, nil
    }
    
    rc.votedFor = proposalID
    return true, nil
}

func (rc *RaftConsensus) RequestVote(ctx context.Context, candidateID string, lastLogIndex, lastLogTerm int) (bool, error) {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
    // 檢查候選人的日誌是否至少和我的一樣新
    myLastLogIndex := len(rc.log)
    myLastLogTerm := 0
    if myLastLogIndex > 0 {
        myLastLogTerm = rc.log[myLastLogIndex-1].Term
    }
    
    if lastLogTerm < myLastLogTerm {
        return false, nil
    }
    
    if lastLogTerm == myLastLogTerm && lastLogIndex < myLastLogIndex {
        return false, nil
    }
    
    // 如果候選人的日誌和我的一樣新或更新，投票給它
    rc.currentTerm++
    rc.votedFor = candidateID
    
    return true, nil
}
```

### 3. 領導者選舉

```go
func (rc *RaftConsensus) startElection() {
    rc.mu.Lock()
    
    rc.state = RaftStateCandidate
    rc.currentTerm++
    rc.votedFor = rc.nodeID
    
    term := rc.currentTerm
    lastLogIndex := len(rc.log)
    lastLogTerm := 0
    if lastLogIndex > 0 {
        lastLogTerm = rc.log[lastLogIndex-1].Term
    }
    
    rc.mu.Unlock()
    
    // 發送請求投票
    votes := 1 // 自己的一票
    
    var wg sync.WaitGroup
    for _, peer := range rc.peers {
        wg.Add(1)
        go func(p *Peer) {
            defer wg.Done()
            
            granted, err := rc.requestVoteFromPeer(p, term, lastLogIndex, lastLogTerm)
            if err != nil {
                return
            }
            
            if granted {
                atomic.AddInt32((*int32)(&votes), 1)
            }
        }(peer)
    }
    wg.Wait()
    
    // 檢查是否獲得多數票
    quorum := len(rc.peers)/2 + 1
    if votes >= quorum {
        rc.mu.Lock()
        rc.state = RaftStateLeader
        rc.mu.Unlock()
        
        // 成為 leader 後立即發送心跳
        go rc.sendHeartbeats()
    }
}

func (rc *RaftConsensus) requestVoteFromPeer(peer *Peer, term, lastLogIndex, lastLogTerm int) (bool, error) {
    // 這裡需要通過網路發送請求
    // 簡化版本，假設有 RPC 客戶端
    return true, nil
}
```

### 4. 日誌複製

```go
func (rc *RaftConsensus) replicateToPeer(peer *Peer, entry LogEntry) error {
    rc.mu.RLock()
    nextIndex := peer.NextIndex
    prevLogIndex := nextIndex - 1
    prevLogTerm := 0
    
    if prevLogIndex > 0 && prevLogIndex <= len(rc.log) {
        prevLogTerm = rc.log[prevLogIndex-1].Term
    }
    
    entries := []LogEntry{entry}
    rc.mu.RUnlock()
    
    // 發送 AppendEntries 請求
    success, err := rc.sendAppendEntries(peer, prevLogIndex, prevLogTerm, entries)
    if err != nil {
        return err
    }
    
    if success {
        rc.mu.Lock()
        peer.NextIndex = prevLogIndex + len(entries) + 1
        peer.MatchIndex = peer.NextIndex - 1
        rc.mu.Unlock()
    } else {
        rc.mu.Lock()
        peer.NextIndex--
        rc.mu.Unlock()
    }
    
    return nil
}

func (rc *RaftConsensus) sendAppendEntries(peer *Peer, prevLogIndex, prevLogTerm int, entries []LogEntry) (bool, error) {
    // RPC 調用
    return true, nil
}
```

### 5. 提交檢查

```go
func (rc *RaftConsensus) maybeCommit() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
    if rc.state != RaftStateLeader {
        return
    }
    
    // 找到最後一個大多數複製的條目
    for n := len(rc.log); n > rc.commitIndex; n-- {
        if rc.log[n-1].Term != rc.currentTerm {
            continue
        }
        
        // 計算有多少節點有這個條目
        count := 1 // leader 自己
        for _, peer := range rc.peers {
            if peer.MatchIndex >= n {
                count++
            }
        }
        
        quorum := len(rc.peers)/2 + 1
        if count >= quorum {
            rc.commitIndex = n
            break
        }
    }
}

func (rc *RaftConsensus) apply CommittedEntries() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
    for i := rc.lastApplied + 1; i <= rc.commitIndex; i++ {
        entry := rc.log[i-1]
        
        // 應用命令到狀態機
        go func(e LogEntry) {
            // 這裡調用具體的狀態機應用邏輯
        }(entry)
        
        rc.lastApplied = i
    }
}
```

## 多數投票共識

```go
type MajorityVoteConsensus struct {
    *MajorityConsensus
    proposalResults map[string]*ProposalResult
}

type ProposalResult struct {
    Value      interface{}
    Votes      map[string]bool
    Committed  bool
}

func (mvc *MajorityVoteConsensus) Propose(ctx context.Context, value interface{}) (*Proposal, error) {
    proposal := &Proposal{
        ID:        generateID(),
        Value:     value,
        Proposer:  mvc.config.NodeID,
        Votes:     make(map[string]bool),
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(mvc.config.Timeout),
    }
    
    mvc.mu.Lock()
    mvc.votes[proposal.ID] = mvc.config.NodeID
    proposal.Votes[mvc.config.NodeID] = true
    mvc.mu.Unlock()
    
    // 廣播提案
    go mvc.broadcastProposal(proposal)
    
    return proposal, nil
}

func (mvc *MajorityVoteConsensus) checkAndCommit(proposalID string) {
    mvc.mu.Lock()
    voteCount := 0
    for _, voted := range mvc.votes {
        if voted == proposalID {
            voteCount++
        }
    }
    
    if voteCount >= mvc.thresholds {
        mvc.proposalResults[proposalID].Committed = true
    }
    mvc.mu.Unlock()
}
```

## 使用範例

### 基本用法

```go
// 創建共識協議
consensus := NewRaftConsensus(ConsensusConfig{
    ProtocolType:     ProtocolTypeRaft,
    NodeID:           "node-1",
    Peers:            []string{"node-2", "node-3"},
    Timeout:          5 * time.Second,
    HeartbeatInterval: 1 * time.Second,
    ElectionTimeout:   10 * time.Second,
})

// 添加觀察者
consensus.AddObserver(myObserver)

// 提交提案
proposal, err := consensus.Propose(ctx, map[string]interface{}{
    "type":  "task_assignment",
    "task":  "task-123",
    "agent": "agent-1",
})

// 等待共識
for {
    committed, _ := consensus.IsConsensus(proposal.ID)
    if committed {
        value, _ := consensus.GetConsensusValue(proposal.ID)
        fmt.Printf("達成共識: %v\n", value)
        break
    }
    time.Sleep(100 * time.Millisecond)
}
```

### 與 SwarmExt 整合

```go
type SwarmExtWithConsensus struct {
    *SwarmExt
    consensus ConsensusProtocol
}

func (s *SwarmExtWithConsensus) AssignTaskWithConsensus(taskID, agentID string) error {
    // 提交分配決策
    proposal, err := s.consensus.Propose(ctx, map[string]interface{}{
        "type":    "task_assignment",
        "task_id": taskID,
        "agent_id": agentID,
    })
    if err != nil {
        return err
    }
    
    // 等待共識
    for {
        committed, _ := s.consensus.IsConsensus(proposal.ID)
        if committed {
            return s.SwarmExt.AssignTask(taskID, agentID)
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(100 * time.Millisecond):
        }
    }
}
```

## 錯誤處理

```go
var (
    ErrNotLeader       = errors.New("node is not the leader")
    ErrProposalExpired  = errors.New("proposal has expired")
    ErrNoConsensus     = errors.New("failed to reach consensus")
    ErrInvalidProposal  = errors.New("invalid proposal")
    ErrStaleTerm       = errors.New("stale term")
    ErrNetworkFailure   = errors.New("network failure")
)
```

## 與其他元件的整合

| 元件 | 整合方式 |
|------|---------|
| SwarmExt | 任務分配決策共識 |
| Guardian | 故障檢測和恢復 |
| StateMachine | 狀態變更共識 |
| DependencyManager | 依賴關係共識 |

## 下一步

- [ ] 實現完整的 Raft 協議
- [ ] 添加 Paxos 協議支援
- [ ] 實現網路分區處理
- [ ] 添加領導者轉移機制
