package backend

// InsertWorkspaceForTest registers ws with b under its current ID and
// path. It is intended for tests in other packages that need to drive
// HTTP handlers against a synthetic workspace without booting a real
// app.App. Production code should go through CreateWorkspace.
func InsertWorkspaceForTest(b *Backend, ws *Workspace) {
	if ws.resolvedPath == "" {
		ws.resolvedPath = ws.Path
	}
	if ws.clients == nil {
		ws.clients = make(map[string]*clientState)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.workspaces.Set(ws.ID, ws)
	if ws.resolvedPath != "" {
		b.pathIndex[ws.resolvedPath] = ws.ID
	}
}
