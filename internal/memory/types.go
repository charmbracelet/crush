package memory

import "time"

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

type Kind string

const (
	KindUser      Kind = "user"
	KindFeedback  Kind = "feedback"
	KindProject   Kind = "project"
	KindReference Kind = "reference"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
	StatusRejected   Status = "rejected"
	StatusDeleted    Status = "deleted"
)

// SessionRecordingMode controls whether a completed session can contribute
// new memories. Recall is intentionally independent from this setting.
type SessionRecordingMode string

const (
	SessionRecordingEnabled  SessionRecordingMode = "enabled"
	SessionRecordingDisabled SessionRecordingMode = "disabled"
	SessionRecordingPolluted SessionRecordingMode = "polluted"
)

type Project struct {
	ID   string
	Name string
	Root string
}

type Observation struct {
	Scope           Scope
	ProjectID       string
	Kind            Kind
	Name            string
	Description     string
	Content         string
	Confidence      float64
	Explicit        bool
	Derivable       bool
	Pinned          bool
	ReplacesID      string
	SourceSessionID string
	SourceMessageID string
	SourceKind      string
	ObservedAt      time.Time
	Status          Status
}

type Record struct {
	ID              string
	Scope           Scope
	ProjectID       string
	Kind            Kind
	Name            string
	Description     string
	Content         string
	Status          Status
	Confidence      float64
	Pinned          bool
	Explicit        bool
	Derivable       bool
	Fingerprint     string
	ReplacesID      string
	FilePath        string
	SourceSessionID string
	SourceMessageID string
	ObservedAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastRecalledAt  time.Time
	RecallCount     int64
}

type Cursor struct {
	CreatedAt int64
	MessageID string
}

type Stats struct {
	Active     int64
	Pending    int64
	Superseded int64
	Rejected   int64
	Deleted    int64
}

type Retrieval struct {
	SessionID string
	ProjectID string
	Query     string
	Selected  []string
	Available int
	Fallback  bool
}

type Options struct {
	Directory             string
	AutoApproveConfidence float64
	MaxRecall             int
	MaxIndexEntries       int
	MaxBackups            int
}

func (o Options) withDefaults() Options {
	if o.AutoApproveConfidence <= 0 || o.AutoApproveConfidence > 1 {
		o.AutoApproveConfidence = 0.88
	}
	if o.MaxRecall <= 0 {
		o.MaxRecall = 5
	}
	if o.MaxIndexEntries <= 0 {
		o.MaxIndexEntries = 80
	}
	if o.MaxBackups <= 0 {
		o.MaxBackups = 5
	}
	return o
}
