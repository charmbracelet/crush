package session

type ModeState struct {
	CollaborationMode CollaborationMode
	PermissionMode    PermissionMode
}

func NormalizeModeState(state ModeState) ModeState {
	return ModeState{
		CollaborationMode: NormalizeCollaborationMode(string(state.CollaborationMode)),
		PermissionMode:    NormalizePermissionMode(string(state.PermissionMode)),
	}
}

func ModeStateFromSession(sess Session) ModeState {
	return NormalizeModeState(ModeState{
		CollaborationMode: sess.CollaborationMode,
		PermissionMode:    sess.PermissionMode,
	})
}

func (s ModeState) WithCollaborationMode(mode CollaborationMode) ModeState {
	s.CollaborationMode = mode
	return NormalizeModeState(s)
}

func (s ModeState) WithPermissionMode(mode PermissionMode) ModeState {
	s.PermissionMode = mode
	return NormalizeModeState(s)
}

func (s ModeState) CurrentModeID() string {
	return string(NormalizePermissionMode(string(s.PermissionMode)))
}

func (s ModeState) IsPlanMode() bool {
	return NormalizeCollaborationMode(string(s.CollaborationMode)) == CollaborationModePlan
}

type ModeTransition struct {
	Previous ModeState
	Current  ModeState
}

func NewModeTransition(current Session, nextCollaborationMode *CollaborationMode, nextPermissionMode *PermissionMode) ModeTransition {
	previous := ModeStateFromSession(current)
	next := previous
	if nextCollaborationMode != nil {
		next = next.WithCollaborationMode(*nextCollaborationMode)
	}
	if nextPermissionMode != nil {
		next = next.WithPermissionMode(*nextPermissionMode)
	}
	return ModeTransition{
		Previous: previous,
		Current:  next,
	}
}

func NewCollaborationModeTransition(current Session, nextMode CollaborationMode) ModeTransition {
	return NewModeTransition(current, &nextMode, nil)
}

func NewPermissionModeTransition(current Session, nextMode PermissionMode) ModeTransition {
	return NewModeTransition(current, nil, &nextMode)
}

func (t ModeTransition) Changed() bool {
	return t.Previous != t.Current
}

func (t ModeTransition) ExitedAutoMode() bool {
	return t.Previous.PermissionMode == PermissionModeAuto && t.Current.PermissionMode != PermissionModeAuto
}
