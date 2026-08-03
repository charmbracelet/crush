package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestWorkflowRun_Basic(t *testing.T) {
	t.Parallel()
	script := `
		local res = agent("hello")
		log("got " .. res)
		return { result = res }
	`
	spawn := func(_ context.Context, _ int, _, prompt string, _ SpawnOpts) (string, error) {
		require.Equal(t, "hello", prompt)
		return "world", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.JSONEq(t, `{"result":"world"}`, res.Value)
	require.Equal(t, []string{"got world"}, res.Logs)
	require.Equal(t, 1, res.AgentCount)
}

func TestWorkflowRun_ReturnTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		script string
		val    string
	}{
		{"return 1", "1"},
		{"return 'foo'", `"foo"`},
		{"return {1, 2}", "[1,2]"},
		{"return {a = 1}", `{"a":1}`},
		{"return nil", ""},
	}
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			t.Parallel()
			res, err := Run(context.Background(), tc.script, nil, Options{})
			require.NoError(t, err)
			if tc.val == "" {
				require.Empty(t, res.Value)
			} else {
				require.JSONEq(t, tc.val, res.Value)
			}
		})
	}
}

func TestWorkflowRun_EmptyTable(t *testing.T) {
	t.Parallel()
	res, err := Run(context.Background(), "return {}", nil, Options{})
	require.NoError(t, err)
	require.JSONEq(t, "[]", res.Value)
}

func TestWorkflowRun_AgentErrors(t *testing.T) {
	t.Parallel()
	t.Run("empty prompt throws", func(t *testing.T) {
		t.Parallel()
		script := `
			local ok, err = pcall(function()
				agent("")
			end)
			if ok then return "fail" end
			return err
		`
		res, err := Run(context.Background(), script, nil, Options{})
		require.NoError(t, err)
		require.Contains(t, res.Value, "prompt cannot be empty")
	})

	t.Run("spawn error throws", func(t *testing.T) {
		t.Parallel()
		script := `
			local ok, err = pcall(function()
				agent("foo")
			end)
			if ok then return "fail" end
			return err
		`
		spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
			return "", errors.New("boom")
		}
		res, err := Run(context.Background(), script, spawn, Options{})
		require.NoError(t, err)
		require.Contains(t, res.Value, "boom")
	})
}

func TestWorkflowRun_Parallel(t *testing.T) {
	t.Parallel()
	script := `
		local res = parallel({
			{prompt = "a", label = "l1"},
			{prompt = "b"}
		})
		return res
	`
	spawn := func(_ context.Context, _ int, label, prompt string, _ SpawnOpts) (string, error) {
		assert.NotEmpty(t, prompt)
		if prompt == "a" {
			assert.Equal(t, "l1", label)
			return "A", nil
		}
		if prompt == "b" {
			assert.Equal(t, "", label)
			return "", errors.New("B_ERR")
		}
		return "", fmt.Errorf("unexpected prompt %q", prompt)
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.Equal(t, 2, res.AgentCount)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Value), &arr))
	require.Len(t, arr, 2)

	require.Equal(t, true, arr[0]["ok"])
	require.Equal(t, "A", arr[0]["value"])
	require.Equal(t, "l1", arr[0]["label"])

	require.Equal(t, false, arr[1]["ok"])
	require.Equal(t, "B_ERR", arr[1]["error"])
}

func TestWorkflowRun_ParallelValidation(t *testing.T) {
	t.Parallel()
	script := `
		local ok, err = pcall(function()
			parallel({{label = "no prompt"}})
		end)
		if ok then return "fail" end
		return err
	`
	var called atomic.Bool
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		called.Store(true)
		return "", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.Contains(t, res.Value, "missing a prompt string")
	require.False(t, called.Load())
}

func TestWorkflowRun_Concurrency(t *testing.T) {
	t.Parallel()
	script := `
		local calls = {}
		for i = 1, 10 do
			calls[i] = {prompt = "p" .. i}
		end
		parallel(calls)
	`
	var active int32
	var maxActive int32
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		v := atomic.AddInt32(&active, 1)
		for {
			cur := atomic.LoadInt32(&maxActive)
			if v <= cur || atomic.CompareAndSwapInt32(&maxActive, cur, v) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return "ok", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{MaxConcurrent: 3, MaxAgents: 20})
	require.NoError(t, err)
	require.Equal(t, 10, res.AgentCount)
	max := int(atomic.LoadInt32(&maxActive))
	require.LessOrEqual(t, max, 3)
	require.GreaterOrEqual(t, max, 2)
}

func TestWorkflowRun_Caps(t *testing.T) {
	t.Parallel()
	script := `
		local i = 0
		local ok, err = pcall(function()
			while i < 200 do
				agent("foo")
				i = i + 1
			end
		end)
		if not ok then log(err) end
		for j = 1, 250 do
			log("log" .. j)
		end
		return i
	`
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		return "ok", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{MaxAgents: 5})
	require.NoError(t, err)
	require.JSONEq(t, "5", res.Value)
	require.Equal(t, 5, res.AgentCount)

	require.Len(t, res.Logs, 201)
	require.Contains(t, res.Logs[0], "limit (5) reached")
	require.Equal(t, "(further logs dropped)", res.Logs[200])
}

func TestWorkflowRun_Cancellation(t *testing.T) {
	t.Parallel()
	script := `while true do end`
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, script, nil, Options{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestWorkflowRun_JSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{"bare", `{"a":1}`, `{"a":1}`, false},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`, false},
		{"wrapped", "Here is it:\n```json\n{\"a\":1}\n```\nDone", `{"a":1}`, false},
		{"garbage", "not json at all", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			val, err := extractJSON(tc.output)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tc.want, val)
			}
		})
	}
}

func TestWorkflowRun_AgentJSON(t *testing.T) {
	t.Parallel()
	script := `
		local res = agent("foo", {json = true})
		return res
	`
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		return "```json\n{\"foo\":\"bar\"}\n```", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.JSONEq(t, `{"foo":"bar"}`, res.Value)
}

func TestWorkflowRun_MaxScript(t *testing.T) {
	t.Parallel()
	script := strings.Repeat("a", MaxScriptBytes+1)
	_, err := Run(context.Background(), script, nil, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum length")
}

func TestWorkflowRun_LoopUntilDry(t *testing.T) {
	t.Parallel()
	script := `
		local items = {}
		for i = 1, 3 do
			local result = agent("Find the next page of results.", {json = true})
			if not result or #result == 0 then break end
			for _, v in ipairs(result) do
				items[#items+1] = v
			end
		end
		return items
	`
	callCount := 0
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		callCount++
		if callCount == 1 {
			return `["a", "b"]`, nil
		}
		if callCount == 2 {
			return `["c"]`, nil
		}
		return `[]`, nil
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.JSONEq(t, `["a","b","c"]`, res.Value)
	require.Equal(t, 3, res.AgentCount)
}

func TestWorkflowRun_Sandbox(t *testing.T) {
	t.Parallel()
	script := `
		return {
			type_os = type(os),
			type_io = type(io),
			type_require = type(require),
			type_debug = type(debug),
			type_dofile = type(dofile),
			type_loadfile = type(loadfile),
			type_print = type(print),
			type_package = type(package),
		}
	`
	res, err := Run(context.Background(), script, nil, Options{})
	require.NoError(t, err)

	var m map[string]string
	require.NoError(t, json.Unmarshal([]byte(res.Value), &m))
	for _, k := range []string{"type_os", "type_io", "type_require", "type_debug", "type_dofile", "type_loadfile", "type_print", "type_package"} {
		require.Equal(t, "nil", m[k], k)
	}
}

func TestWorkflowRun_CyclicTable(t *testing.T) {
	t.Parallel()
	script := `
		local t = {}
		t.self = t
		return t
	`
	_, err := Run(context.Background(), script, nil, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cyclic")
}

func TestWorkflowRun_NaN(t *testing.T) {
	t.Parallel()
	script := `return {bad = 0/0}`
	_, err := Run(context.Background(), script, nil, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "NaN")
}

func TestWorkflowRun_Inf(t *testing.T) {
	t.Parallel()
	script := `return {inf = 1/0}`
	_, err := Run(context.Background(), script, nil, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Inf")
}

func TestWorkflowRun_Timeout(t *testing.T) {
	t.Parallel()
	script := `while true do end`
	start := time.Now()
	_, err := Run(context.Background(), script, nil, Options{Timeout: 100 * time.Millisecond})
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestWorkflowRun_ParallelJSONExtractionError(t *testing.T) {
	t.Parallel()
	script := `
		local res = parallel({
			{prompt = "a", json = true}
		})
		return res
	`
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		return "not json at all", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Value), &arr))
	require.Len(t, arr, 1)
	require.Equal(t, false, arr[0]["ok"])
	require.Contains(t, arr[0]["error"].(string), "no JSON")
}

func TestWorkflowRun_AgentOptsPassthrough(t *testing.T) {
	t.Parallel()
	script := `
		agent("do-stuff", {model = "small", agent = "coder"})
	`
	var got SpawnOpts
	spawn := func(_ context.Context, _ int, _, _ string, opts SpawnOpts) (string, error) {
		got = opts
		return "ok", nil
	}
	_, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.Equal(t, "small", got.Model)
	require.Equal(t, "coder", got.Agent)
}

func TestWorkflowRun_ParallelOptsPassthrough(t *testing.T) {
	t.Parallel()
	script := `
		parallel({
			{prompt = "a", model = "large", agent = "task"},
			{prompt = "b", agent = "coder"}
		})
	`
	var mu sync.Mutex
	got := map[string]SpawnOpts{}
	spawn := func(_ context.Context, _ int, _, prompt string, opts SpawnOpts) (string, error) {
		mu.Lock()
		got[prompt] = opts
		mu.Unlock()
		return "ok", nil
	}
	_, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.Equal(t, SpawnOpts{Model: "large", Agent: "task"}, got["a"])
	require.Equal(t, SpawnOpts{Agent: "coder"}, got["b"])
}

func TestWorkflowRun_InvalidModel(t *testing.T) {
	t.Parallel()
	script := `
		local ok, err = pcall(function()
			agent("do-stuff", {model = "medium"})
		end)
		if ok then return "fail" end
		return err
	`
	res, err := Run(context.Background(), script, nil, Options{})
	require.NoError(t, err)
	require.Contains(t, res.Value, "invalid model")
	require.Contains(t, res.Value, "large, small")
}

func TestWorkflowRun_InvalidAgent(t *testing.T) {
	t.Parallel()
	script := `
		local ok, err = pcall(function()
			agent("do-stuff", {agent = "planner"})
		end)
		if ok then return "fail" end
		return err
	`
	res, err := Run(context.Background(), script, nil, Options{})
	require.NoError(t, err)
	require.Contains(t, res.Value, "invalid agent")
	require.Contains(t, res.Value, "task, coder")
}

// TestWorkflowRun_CoderParallelSerialised asserts safety 3b: a parallel
// batch containing any agent="coder" entry must never run two agents at
// the same time, even when MaxConcurrent is > 1.
func TestWorkflowRun_CoderParallelSerialised(t *testing.T) {
	t.Parallel()
	script := `
		local calls = {}
		for i = 1, 5 do
			calls[i] = {prompt = "p" .. i, agent = "coder"}
		end
		parallel(calls)
	`
	var active int32
	var maxActive int32
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		v := atomic.AddInt32(&active, 1)
		for {
			cur := atomic.LoadInt32(&maxActive)
			if v <= cur || atomic.CompareAndSwapInt32(&maxActive, cur, v) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return "ok", nil
	}
	res, err := Run(context.Background(), script, spawn, Options{MaxConcurrent: 5, MaxAgents: 10})
	require.NoError(t, err)
	require.Equal(t, 5, res.AgentCount)
	require.Equal(t, int32(1), atomic.LoadInt32(&maxActive),
		"coder batch must serialise: max concurrent should be 1")
}

func TestWorkflowRun_DefaultsPreserved(t *testing.T) {
	t.Parallel()
	script := `
		agent("plain-call")
	`
	var got SpawnOpts
	spawn := func(_ context.Context, _ int, _, _ string, opts SpawnOpts) (string, error) {
		got = opts
		return "ok", nil
	}
	_, err := Run(context.Background(), script, spawn, Options{})
	require.NoError(t, err)
	require.Equal(t, "", got.Model, "default model should be empty (inherit)")
	require.Equal(t, "", got.Agent, "default agent should be empty (task)")
}

func TestWorkflowRun_NumericKeyTables(t *testing.T) {
	t.Parallel()
	// Only a true 1..n sequence may serialize as an array. Sparse or
	// non-positive-integer numeric keys must become objects: the array branch
	// walks 1..Len() and would otherwise drop values or invent nulls.
	cases := []struct{ script, want string }{
		{`return {1, 2, 3}`, `[1,2,3]`},
		{`return {}`, `[]`},
		{`return {[0]="z"}`, `{"0":"z"}`},
		{`return {[-1]="n"}`, `{"-1":"n"}`},
		{`return {[1.5]="f"}`, `{"1.5":"f"}`},
		{`return {[5]="x"}`, `{"5":"x"}`},
		{`return {[1]="a",[3]="c"}`, `{"1":"a","3":"c"}`},
		{`return {1, 2, [10]=99}`, `{"1":1,"2":2,"10":99}`},
		{`return {[1]="a",[2]="b",name="x"}`, `{"1":"a","2":"b","name":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			t.Parallel()
			res, err := Run(context.Background(), tc.script, nil, Options{})
			require.NoError(t, err)
			require.JSONEq(t, tc.want, res.Value)
		})
	}
}

func TestWorkflowRun_ErrorLineNumbers(t *testing.T) {
	t.Parallel()
	// Reported lines must match the script the caller wrote, and the rest of
	// the message must survive intact -- the model reads it to self-correct.
	cases := []struct{ name, script, want string }{
		{"syntax first line", `local x = = 1`, `<string> line:1(column:11) near '=':`},
		{"syntax later line", "\n\nlocal y = = 2", `<string> line:3(column:11) near '=':`},
		{"runtime first line", `error('boom')`, `<string>:1: boom`},
		{"runtime later line", "\n\nerror('deep')", `<string>:3: deep`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Run(context.Background(), tc.script, nil, Options{})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestStripLineOffset_LeavesUnknownShapesAlone(t *testing.T) {
	t.Parallel()
	// Anything that is not a gopher-lua line-bearing message must pass through
	// byte-for-byte rather than being spliced on whatever colons it contains.
	for _, msg := range []string{
		"context deadline exceeded",
		"http://example.com:8080: dial failed",
		"script too large: 65536 bytes",
	} {
		require.Equal(t, msg, stripLineOffset(errors.New(msg)).Error())
	}
	require.NoError(t, stripLineOffset(nil))
}

func TestWorkflowRun_ProgressEvents(t *testing.T) {
	t.Parallel()
	script := `
		log("starting")
		local r1 = agent("a")
		log("mid")
		local r2 = agent("b")
		local r3 = agent("c")
		return r1 .. r2 .. r3
	`
	var mu sync.Mutex
	var events []Progress
	spawn := func(_ context.Context, _ int, _, prompt string, _ SpawnOpts) (string, error) {
		if prompt == "b" {
			// Slightly different delay to verify ordering.
			time.Sleep(5 * time.Millisecond)
		}
		return prompt, nil
	}
	res, err := Run(context.Background(), script, spawn, Options{
		Progress: func(p Progress) {
			mu.Lock()
			events = append(events, p)
			mu.Unlock()
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `"abc"`, res.Value)
	require.Equal(t, 3, res.AgentCount)

	mu.Lock()
	defer mu.Unlock()

	// We expect events in this order:
	//   log("starting")      → kind=log,   running=0, completed=0, total=0
	//   agent("a") start     → kind=agent_start, running=1, completed=0, total=1
	//   agent("a") done      → kind=agent_done,  running=0, completed=1, total=1
	//   log("mid")           → kind=log,   running=0, completed=1, total=1
	//   agent("b") start     → kind=agent_start, running=1, completed=1, total=2
	//   agent("b") done      → kind=agent_done,  running=0, completed=2, total=2
	//   agent("c") start     → kind=agent_start, running=1, completed=2, total=3
	//   agent("c") done      → kind=agent_done,  running=0, completed=3, total=3

	require.Len(t, events, 8, "script emits exactly 8 progress events")

	idx := 0
	// event 0: log("starting")
	require.Equal(t, "log", events[idx].Kind)
	require.Equal(t, -1, events[idx].Index)
	require.Contains(t, events[idx].Message, "starting")
	require.Zero(t, events[idx].Running)
	require.Zero(t, events[idx].Completed)
	require.Zero(t, events[idx].Total)
	idx++

	// event 1: agent("a") start
	require.Equal(t, "agent_start", events[idx].Kind)
	require.Equal(t, 0, events[idx].Index)
	require.Equal(t, 1, events[idx].Running)
	require.Equal(t, 0, events[idx].Completed)
	require.Equal(t, 1, events[idx].Total)
	idx++

	// event 2: agent("a") done
	require.Equal(t, "agent_done", events[idx].Kind)
	require.Equal(t, 0, events[idx].Index)
	require.Equal(t, 0, events[idx].Running)
	require.Equal(t, 1, events[idx].Completed)
	require.Equal(t, 1, events[idx].Total)
	idx++

	// event 3: log("mid")
	require.Equal(t, "log", events[idx].Kind)
	require.Equal(t, -1, events[idx].Index)
	require.Equal(t, 0, events[idx].Running)
	require.Equal(t, 1, events[idx].Completed)
	require.Equal(t, 1, events[idx].Total)
	idx++

	// event 4: agent("b") start
	require.Equal(t, "agent_start", events[idx].Kind)
	require.Equal(t, 1, events[idx].Index)
	require.Equal(t, 1, events[idx].Running)
	require.Equal(t, 1, events[idx].Completed)
	require.Equal(t, 2, events[idx].Total)
	// Label is empty
	require.Empty(t, events[idx].Label)
	idx++

	// event 5: agent("b") done
	require.Equal(t, "agent_done", events[idx].Kind)
	require.Equal(t, 1, events[idx].Index)
	require.Equal(t, 0, events[idx].Running)
	require.Equal(t, 2, events[idx].Completed)
	require.Equal(t, 2, events[idx].Total)
	idx++

	// event 6: agent("c") start
	require.Equal(t, "agent_start", events[idx].Kind)
	require.Equal(t, 2, events[idx].Index)
	require.Equal(t, 1, events[idx].Running)
	require.Equal(t, 2, events[idx].Completed)
	require.Equal(t, 3, events[idx].Total)
	idx++

	// event 7: agent("c") done
	require.Equal(t, "agent_done", events[idx].Kind)
	require.Equal(t, 2, events[idx].Index)
	require.Equal(t, 0, events[idx].Running)
	require.Equal(t, 3, events[idx].Completed)
	require.Equal(t, 3, events[idx].Total)
}

func TestWorkflowRun_CancelParentContext(t *testing.T) {
	t.Parallel()
	script := `
		local calls = {}
		for i = 1, 20 do
			calls[i] = {prompt = "p" .. i}
		end
		parallel(calls)
		return "done"
	`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := make(chan struct{})
	spawn := func(_ context.Context, _ int, _, _ string, _ SpawnOpts) (string, error) {
		<-block
		return "ok", nil
	}

	// Let one agent start, then cancel.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
		close(block)
	}()

	start := time.Now()
	_, err := Run(ctx, script, spawn, Options{MaxConcurrent: 1})
	require.Less(t, time.Since(start), 5*time.Second, "must return promptly after cancel")
	require.ErrorIs(t, err, context.Canceled)
}

// TestWorkflowRun_LogsSurviveFailure guards the failure path returning a
// partial Result. Run used to return a zero Result on every error, so
// workflow_tool.go always rendered "Logs:\n(none)" and dropped the logs
// metadata -- discarding exactly the log() output needed to diagnose the
// failure.
func TestWorkflowRun_LogsSurviveFailure(t *testing.T) {
	t.Parallel()

	spawn := func(_ context.Context, _ int, _, prompt string, _ SpawnOpts) (string, error) {
		return prompt, nil
	}

	t.Run("runtime error", func(t *testing.T) {
		t.Parallel()
		res, err := Run(context.Background(), `
			log("step 1 done")
			agent("a")
			log("step 2 done")
			error("boom")
		`, spawn, Options{})
		require.Error(t, err)
		require.Equal(t, []string{"step 1 done", "step 2 done"}, res.Logs,
			"logs emitted before the error must survive the failure")
		require.Equal(t, 1, res.AgentCount, "agent count must survive the failure")
	})

	t.Run("spawn error", func(t *testing.T) {
		t.Parallel()
		res, err := Run(context.Background(), `
			log("before spawn")
			agent("a")
		`, func(context.Context, int, string, string, SpawnOpts) (string, error) {
			return "", errors.New("spawn exploded")
		}, Options{})
		require.Error(t, err)
		require.Equal(t, []string{"before spawn"}, res.Logs)
		require.Equal(t, 1, res.AgentCount)
	})

	t.Run("unserializable return value", func(t *testing.T) {
		t.Parallel()
		res, err := Run(context.Background(), `
			log("computed")
			local t = {}
			t.self = t
			return t
		`, spawn, Options{})
		require.Error(t, err)
		require.Equal(t, []string{"computed"}, res.Logs)
	})
}

// TestWorkflowRun_LogTruncationKeepsValidUTF8 guards addLog's rune-boundary
// truncation. A raw msg[:maxLogEntryBytes] can split a multi-byte rune, and
// json.Marshal then rewrites the partial sequence to U+FFFD in the tool
// metadata the model reads.
func TestWorkflowRun_LogTruncationKeepsValidUTF8(t *testing.T) {
	t.Parallel()

	// 3-byte runes: 2048 is not a multiple of 3, so a byte-offset cut splits one.
	// The literal is a real U+20AC in the source; gopher-lua is 5.1 and has no
	// \u{...} escape, so an escape here would silently degrade to ASCII and the
	// test would pass without exercising the truncation at all.
	res, err := Run(context.Background(), `log(string.rep("€", 1000))`,
		func(context.Context, int, string, string, SpawnOpts) (string, error) { return "", nil },
		Options{})
	require.NoError(t, err)
	require.Len(t, res.Logs, 1)

	entry := res.Logs[0]
	require.LessOrEqual(t, len(entry), maxLogEntryBytes, "entry must still be capped")
	require.Greater(t, len(entry), maxLogEntryBytes-4, "entry must not lose more than one rune")
	require.True(t, utf8.ValidString(entry), "truncation must not split a rune")

	b, err := json.Marshal(entry)
	require.NoError(t, err)
	require.NotContains(t, string(b), `�`, "no replacement chars must reach the tool metadata")
}

func TestTruncateUTF8(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"under the limit", "abc", 10, "abc"},
		{"exactly the limit", "abc", 3, "abc"},
		{"ascii cut", "abcdef", 3, "abc"},
		{"cut on a rune boundary", "€€", 3, "€"},
		{"cut mid rune backs off", "€€", 4, "€"},
		{"cut mid rune backs off further", "€€", 5, "€"},
		{"limit zero", "€", 0, ""},
		{"whole string is one oversized rune", "\U0001F600", 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateUTF8(tc.in, tc.limit)
			require.Equal(t, tc.want, got)
			require.True(t, utf8.ValidString(got))
		})
	}
}

// TestParallelCoderDoesNotStarveReadOnlyTasks pins the lock-order fix: with
// MaxConcurrent=2 and a mixed batch of 2 coders + 2 tasks, both tasks must
// start while the coder is still blocked on the batch lock. The old code
// acquired the global semaphore first, so blocked coders consumed the global
// slots and the read-only tasks never started.
//
// GOMAXPROCS is pinned to 1 so goroutine scheduling is strictly FIFO: the
// first coder goroutine (created first) acquires a semaphore slot before the
// tasks ever run. Without the pin the task goroutines can win a slot early,
// finish, and mask the starvation on the old code.
func TestParallelCoderDoesNotStarveReadOnlyTasks(t *testing.T) {
	oldMaxProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(oldMaxProcs) })

	script := `
		local res = parallel({
			{prompt = "c1", agent = "coder"},
			{prompt = "c2", agent = "coder"},
			{prompt = "t1"},
			{prompt = "t2"}
		})
		return res
	`
	var mu sync.Mutex
	started := map[int]bool{}
	coderGate := make(chan struct{})
	spawn := func(_ context.Context, index int, _, prompt string, opts SpawnOpts) (string, error) {
		mu.Lock()
		started[index] = true
		isCoder := opts.Agent == "coder"
		mu.Unlock()
		if isCoder {
			<-coderGate // hold every coder blocked until the test releases it
		}
		return "ok", nil
	}

	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), script, spawn, Options{MaxConcurrent: 2, MaxAgents: 10})
		done <- err
	}()

	// With GOMAXPROCS=1 the first coder (index 0) runs first: it takes the
	// batch lock, a global slot, and blocks on the gate. The second coder is
	// queued on the batch lock and the tasks are queued on the free global
	// slot. Wait for coder 0 to be blocked before checking the tasks.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return started[0]
	}, 2*time.Second, 10*time.Millisecond)

	// Tasks are entries 3 and 4, i.e. global indices 2 and 3 (reserveIndices
	// assigns sequentially from 0). With the coder blocked they must still
	// start.
	tasksStarted := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		t1, t2 := started[2], started[3]
		mu.Unlock()
		if t1 && t2 {
			tasksStarted = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(coderGate) // release coders so the run can finish
	require.NoError(t, <-done)
	require.True(t, tasksStarted,
		"read-only tasks must start while coders are serialised; blocked coders starved them")
}

// TestParallelMixedBatchInterleavesTaskBetweenCoders asserts both halves of
// the fix at once: the two coders never overlap (Safety 3b still holds) and a
// read-only task runs concurrently with a coder (the batch lock is not
// applied to the whole batch).
func TestParallelMixedBatchInterleavesTaskBetweenCoders(t *testing.T) {
	t.Parallel()
	script := `
		local res = parallel({
			{prompt = "c1", agent = "coder"},
			{prompt = "c2", agent = "coder"},
			{prompt = "t1"},
			{prompt = "t2"}
		})
		return res
	`
	type interval struct{ start, end time.Time }
	var mu sync.Mutex
	coderIntervals := map[int]interval{}
	taskStarts := map[int]time.Time{}
	spawn := func(_ context.Context, index int, _, prompt string, opts SpawnOpts) (string, error) {
		start := time.Now()
		if opts.Agent == "coder" {
			mu.Lock()
			coderIntervals[index] = interval{start: start}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			iv := coderIntervals[index]
			iv.end = time.Now()
			coderIntervals[index] = iv
			mu.Unlock()
		} else {
			mu.Lock()
			taskStarts[index] = start
			mu.Unlock()
		}
		return "ok", nil
	}

	res, err := Run(context.Background(), script, spawn, Options{MaxConcurrent: 2, MaxAgents: 10})
	require.NoError(t, err)
	require.Equal(t, 4, res.AgentCount)

	mu.Lock()
	c1, c2 := coderIntervals[0], coderIntervals[1]
	t1, t2 := taskStarts[2], taskStarts[3]
	mu.Unlock()

	require.True(t, !c1.end.IsZero() && !c2.end.IsZero(), "both coders must have run")
	// Safety 3b: the two coder executions must not overlap.
	if c1.start.Before(c2.start) {
		require.False(t, c2.start.Before(c1.end), "coder executions must not overlap")
	} else {
		require.False(t, c1.start.Before(c2.end), "coder executions must not overlap")
	}
	// A read-only task must have run concurrently with a coder.
	concurrent := (!t1.IsZero() && !t1.Before(c1.start) && t1.Before(c1.end)) ||
		(!t2.IsZero() && !t2.Before(c1.start) && t2.Before(c1.end)) ||
		(!t1.IsZero() && !t1.Before(c2.start) && t1.Before(c2.end)) ||
		(!t2.IsZero() && !t2.Before(c2.start) && t2.Before(c2.end))
	require.True(t, concurrent, "a read-only task must run while a coder is executing")
}
