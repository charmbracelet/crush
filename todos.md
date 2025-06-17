## TODOs before release

- [x] Implement help
  - [x] Show full help
  - [x] Make help dependent on the focused pane and page
- [x] Implement current model in the sidebar
- [x] Implement LSP errors
- [x] Implement changed files
  - [x] Implement initial load
  - [x] Implement realtime file changes
- [ ] Events when tool error
- [ ] Support bash commands
- [ ] Editor attachments fixes
  - [ ] Reimplement removing attachments
- [ ] Fix the logs view
  - [ ] Review the implementation
  - [ ] The page lags
  - [ ] Make the logs long lived ?
- [ ] Add all possible actions to the commands
- [ ] Parallel tool calls and permissions
  - [ ] Run the tools in parallel and add results in parallel
  - [ ] Show multiple permissions dialogs
- [ ] Investigate messages issues
  - [ ] Weird behavior sometimes the message does not update
  - [ ] Message length (I saw the message go beyond the correct length when there are errors)
  - [ ] Address UX issues
  - [ ] Fix issue with numbers (padding) view tool
- [ ] Implement responsive mode
- [ ] Update interactive mode to use the spinner
- [ ] Revisit the core list component
  - [ ] This component has become super complex we might need to fix this.
- [ ] Handle correct LSP and MCP status icon
- [x] Investigate ways to make the spinner less CPU intensive
- [ ] General cleanup and documentation
- [ ] Update the readme

## Maybe

- [ ] Revisit the provider/model/configs
- [ ] Implement correct persistent shell
- [ ] Store file read/write time somewhere so that the we can make sure that even if we restart we do not need to re-read the same file
- [ ] Send updates to the UI when new LSP diagnostics are available
