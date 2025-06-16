## TODOs before release

- [x] Implement help
  - [x] Show full help
  - [x] Make help dependent on the focused pane and page
- [ ] Implement current model in the sidebar
- [ ] Implement changed files
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
- [ ] Revisit the core list component
  - [ ] This component has become super complex we might need to fix this.
- [ ] Investigate ways to make the spinner less CPU intensive
- [ ] General cleanup and documentation
- [ ] Update the readme

## Maybe

- [ ] Revisit the provider/model/configs
- [ ] Implement correct persistent shell
- [ ] Store file read/write time somewhere so that the we can make sure that even if we restart we do not need to re-read the same file
