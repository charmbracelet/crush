# charm.land/fantasy usage map

This file captures repository usage and dependency points for `charm.land/fantasy` and related provider packages.


## Usage references
- AGENTS.md#L50-L50
- Taskfile.yaml#L195-L195
- go.mod#L10-L10
- go.sum#L9-L10
- internal/agent/agent.go#L26-L32, internal/agent/agent.go#L72-L110, internal/agent/agent.go#L133-L133, internal/agent/agent.go#L157-L157, internal/agent/agent.go#L202-L206, internal/agent/agent.go#L252-L263, internal/agent/agent.go#L288-L328, internal/agent/agent.go#L371-L442, internal/agent/agent.go#L514-L515, internal/agent/agent.go#L538-L539, internal/agent/agent.go#L593-L593, internal/agent/agent.go#L622-L653, internal/agent/agent.go#L707-L711, internal/agent/agent.go#L741-L769, internal/agent/agent.go#L816-L830, internal/agent/agent.go#L921-L934, internal/agent/agent.go#L1042-L1073, internal/agent/agent.go#L1108-L1166
- internal/agent/agent_test.go#L11-L29
- internal/agent/agent_tool.go#L8-L55
- internal/agent/agentic_fetch_tool.go#L12-L12, internal/agent/agentic_fetch_tool.go#L53-L72, internal/agent/agentic_fetch_tool.go#L95-L166
- internal/agent/common_test.go#L12-L16, internal/agent/common_test.go#L45-L93, internal/agent/common_test.go#L141-L141, internal/agent/common_test.go#L168-L168, internal/agent/common_test.go#L210-L210
- internal/agent/coordinator.go#L19-L63, internal/agent/coordinator.go#L136-L136, internal/agent/coordinator.go#L177-L177, internal/agent/coordinator.go#L215-L216, internal/agent/coordinator.go#L370-L370, internal/agent/coordinator.go#L422-L423, internal/agent/coordinator.go#L478-L478, internal/agent/coordinator.go#L508-L508, internal/agent/coordinator.go#L598-L598, internal/agent/coordinator.go#L630-L676, internal/agent/coordinator.go#L705-L705, internal/agent/coordinator.go#L728-L800, internal/agent/coordinator.go#L923-L923, internal/agent/coordinator.go#L970-L1017
- internal/agent/coordinator_test.go#L9-L91, internal/agent/coordinator_test.go#L127-L127, internal/agent/coordinator_test.go#L198-L198, internal/agent/coordinator_test.go#L224-L224, internal/agent/coordinator_test.go#L250-L250
- internal/agent/event.go#L6-L26
- internal/agent/hyper/provider.go#L1-L1, internal/agent/hyper/provider.go#L25-L26, internal/agent/hyper/provider.go#L55-L55, internal/agent/hyper/provider.go#L86-L88, internal/agent/hyper/provider.go#L128-L174, internal/agent/hyper/provider.go#L195-L222, internal/agent/hyper/provider.go#L259-L275, internal/agent/hyper/provider.go#L324-L325
- internal/agent/hyper/provider.json#L1-L1
- internal/agent/loop_detection.go#L8-L19, internal/agent/loop_detection.go#L45-L52, internal/agent/loop_detection.go#L75-L88
- internal/agent/loop_detection_test.go#L7-L197
- internal/agent/tools/bash.go#L15-L15, internal/agent/tools/bash.go#L191-L302, internal/agent/tools/bash.go#L332-L375
- internal/agent/tools/bash_test.go#L8-L8, internal/agent/tools/bash_test.go#L82-L94
- internal/agent/tools/diagnostics.go#L13-L37
- internal/agent/tools/download.go#L15-L15, internal/agent/tools/download.go#L37-L146
- internal/agent/tools/edit.go#L13-L13, internal/agent/tools/edit.go#L47-L79, internal/agent/tools/edit.go#L110-L128, internal/agent/tools/edit.go#L152-L243, internal/agent/tools/edit.go#L271-L354, internal/agent/tools/edit.go#L378-L378, internal/agent/tools/edit.go#L402-L442
- internal/agent/tools/fetch.go#L13-L168
- internal/agent/tools/glob.go#L15-L62
- internal/agent/tools/grep.go#L21-L21, internal/agent/tools/grep.go#L105-L126, internal/agent/tools/grep.go#L164-L165
- internal/agent/tools/job_kill.go#L8-L8, internal/agent/tools/job_kill.go#L29-L57
- internal/agent/tools/job_output.go#L9-L9, internal/agent/tools/job_output.go#L33-L45, internal/agent/tools/job_output.go#L88-L88
- internal/agent/tools/list_mcp_resources.go#L11-L70, internal/agent/tools/list_mcp_resources.go#L96-L96
- internal/agent/tools/ls.go#L12-L12, internal/agent/tools/ls.go#L58-L114
- internal/agent/tools/lsp_restart.go#L12-L42, internal/agent/tools/lsp_restart.go#L74-L77
- internal/agent/tools/mcp-tools.go#L8-L8, internal/agent/tools/mcp-tools.go#L47-L70, internal/agent/tools/mcp-tools.go#L91-L148
- internal/agent/tools/multiedit.go#L13-L13, internal/agent/tools/multiedit.go#L66-L86, internal/agent/tools/multiedit.go#L126-L143, internal/agent/tools/multiedit.go#L168-L168, internal/agent/tools/multiedit.go#L195-L277, internal/agent/tools/multiedit.go#L302-L310, internal/agent/tools/multiedit.go#L337-L358, internal/agent/tools/multiedit.go#L384-L385
- internal/agent/tools/read_mcp_resource.go#L11-L11, internal/agent/tools/read_mcp_resource.go#L33-L99
- internal/agent/tools/references.go#L17-L57, internal/agent/tools/references.go#L83-L89
- internal/agent/tools/sourcegraph.go#L14-L51, internal/agent/tools/sourcegraph.go#L90-L136
- internal/agent/tools/todos.go#L8-L8, internal/agent/tools/todos.go#L36-L61, internal/agent/tools/todos.go#L101-L101, internal/agent/tools/todos.go#L132-L132
- internal/agent/tools/view.go#L17-L17, internal/agent/tools/view.go#L68-L122, internal/agent/tools/view.go#L148-L200, internal/agent/tools/view.go#L228-L229, internal/agent/tools/view.go#L384-L395, internal/agent/tools/view.go#L430-L431
- internal/agent/tools/web_fetch.go#L12-L71
- internal/agent/tools/web_search.go#L10-L53
- internal/agent/tools/write.go#L13-L13, internal/agent/tools/write.go#L52-L95, internal/agent/tools/write.go#L128-L145, internal/agent/tools/write.go#L168-L168
- internal/app/app.go#L19-L19, internal/app/app.go#L291-L291
- internal/message/content.go#L12-L15, internal/message/content.go#L463-L560
