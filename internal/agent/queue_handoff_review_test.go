package agent

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// ---------- helpers (từ lượt soát độc lập 15/10) ----------

type probeModel struct {
	mu      sync.Mutex
	prompts []string
	starts  map[string]int
	gates   map[string]chan struct{}
	entered map[string]chan struct{}
	failOn  map[string]error
	delay   time.Duration
	onStart func(prompt string, ctxAlive bool)
}

func newProbeModel() *probeModel {
	return &probeModel{
		starts:  map[string]int{},
		gates:   map[string]chan struct{}{},
		entered: map[string]chan struct{}{},
		failOn:  map[string]error{},
	}
}

func (m *probeModel) gate(p string) chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := make(chan struct{})
	m.gates[p] = g
	m.entered[p] = make(chan struct{})
	return g
}
func (m *probeModel) enteredCh(p string) chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entered[p]
}
func (m *probeModel) fail(p string, err error) { m.mu.Lock(); m.failOn[p] = err; m.mu.Unlock() }
func (m *probeModel) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.prompts...)
}

func (m *probeModel) Provider() string { return "fake" }
func (m *probeModel) Model() string    { return "fake-model" }
func (m *probeModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "title"}}, FinishReason: fantasy.FinishReasonStop}, nil
}
func (m *probeModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("na")
}
func (m *probeModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("na")
}

func lastUserText(p fantasy.Prompt) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i].Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range p[i].Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				return tp.Text
			}
		}
	}
	return ""
}

func (m *probeModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	prompt := lastUserText(call.Prompt)
	m.mu.Lock()
	m.prompts = append(m.prompts, prompt)
	m.starts[prompt]++
	g := m.gates[prompt]
	en := m.entered[prompt]
	ferr := m.failOn[prompt]
	hook := m.onStart
	m.mu.Unlock()
	if hook != nil {
		hook(prompt, ctx.Err() == nil)
	}
	if en != nil {
		func() {
			defer func() { _ = recover() }() // already closed on a second call
			close(en)
		}()
	}
	if g != nil {
		select {
		case <-g:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if ferr != nil {
		return nil, ferr
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "done:" + prompt}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

type rcCollector struct {
	mu  sync.Mutex
	got map[string][]notify.RunComplete
}

func newRCCollector(ctx context.Context, b *pubsub.Broker[notify.RunComplete]) *rcCollector {
	c := &rcCollector{got: map[string][]notify.RunComplete{}}
	ch := b.Subscribe(ctx)
	go func() {
		for ev := range ch {
			c.mu.Lock()
			c.got[ev.Payload.RunID] = append(c.got[ev.Payload.RunID], ev.Payload)
			c.mu.Unlock()
		}
	}()
	return c
}
func (c *rcCollector) of(id string) []notify.RunComplete {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]notify.RunComplete(nil), c.got[id]...)
}
func (c *rcCollector) waitN(t *testing.T, id string, n int, d time.Duration) []notify.RunComplete {
	t.Helper()
	dl := time.Now().Add(d)
	for time.Now().Before(dl) {
		if got := c.of(id); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return c.of(id)
}

func probeAgent(t *testing.T, large fantasy.LanguageModel) (*sessionAgent, *pubsub.Broker[notify.RunComplete], string, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	cw := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: cw},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: cw},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	return sa, broker, sess.ID, env
}

func (a *sessionAgent) probeInternals(sid string) string {
	a.acceptedMu.Lock()
	cnt, hasCnt := a.acceptedRuns.Get(sid)
	a.acceptedMu.Unlock()
	mark, hasMark := a.cancelMark.Get(sid)
	_, hasActive := a.activeRequests.Get(sid)
	return fmt.Sprintf("queued=%d busy=%v acceptedRuns=%d(%v) cancelMark=%d(%v)", a.QueuedPrompts(sid), hasActive, cnt, hasCnt, mark, hasMark)
}

func (a *sessionAgent) probeClean(sid string) bool {
	a.acceptedMu.Lock()
	_, hasCnt := a.acceptedRuns.Get(sid)
	a.acceptedMu.Unlock()
	_, hasMark := a.cancelMark.Get(sid)
	return a.QueuedPrompts(sid) == 0 && !a.IsSessionBusy(sid) && !hasCnt && !hasMark
}

func bg() context.Context { return context.Background() }

// Lượt lỗi provider vẫn chuyển tiếp hàng đợi: prompt phía sau chạy và có RunComplete.

func TestErrorTurnHandsOffQueue(t *testing.T) {
	m := newProbeModel()
	g := m.gate("a")
	m.fail("a", errors.New("provider exploded"))
	sa, broker, sid, _ := probeAgent(t, m)
	col := newRCCollector(t.Context(), broker)

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "a", Prompt: "a"})
		done <- err
	}()
	waitEntered(t, m.enteredCh("a"))
	_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "q", Prompt: "q"})
	require.NoError(t, err)
	close(g)
	runErr := <-done
	time.Sleep(500 * time.Millisecond)
	t.Logf("run a err=%v; prompts=%v; q RunCompletes=%d; state: %s", runErr, m.seen(), len(col.of("q")), sa.probeInternals(sid))
	require.Len(t, col.of("q"), 1, "queued prompt q never got a RunComplete after run a failed")
}

// CancelRun lượt đang chạy: hàng đợi chạy tiếp đúng thứ tự, mỗi RunID một RunComplete.

func TestCancelRunHandsOffInOrder(t *testing.T) {
	m := newProbeModel()
	m.gate("a")
	sa, broker, sid, _ := probeAgent(t, m)
	col := newRCCollector(t.Context(), broker)
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "a", Prompt: "a"})
		done <- err
	}()
	waitEntered(t, m.enteredCh("a"))
	for _, id := range []string{"q1", "q2", "q3"} {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: id, Prompt: id})
		require.NoError(t, err)
	}
	require.Equal(t, 3, sa.QueuedPrompts(sid))
	sa.CancelRun(sid, "a")
	select {
	case err := <-done:
		t.Logf("Run(a) returned err=%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run(a) did not return")
	}
	for _, id := range []string{"a", "q1", "q2", "q3"} {
		got := col.waitN(t, id, 1, 3*time.Second)
		time.Sleep(50 * time.Millisecond)
		got = col.of(id)
		require.Lenf(t, got, 1, "RunComplete count for %s: %+v", id, got)
	}
	require.True(t, col.of("a")[0].Cancelled)
	for _, id := range []string{"q1", "q2", "q3"} {
		require.False(t, col.of(id)[0].Cancelled, id)
		require.Empty(t, col.of(id)[0].Error, id)
	}
	require.Equal(t, []string{"a", "q1", "q2", "q3"}, m.seen())
	require.True(t, sa.probeClean(sid), sa.probeInternals(sid))
}

// CancelRun lúc đang tự tóm tắt: dừng lượt, chuyển tiếp hàng đợi, mỗi RunID một RunComplete.

func TestCancelRunDuringSummarizeHandsOff(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &summarizeGateModel{entered: make(chan struct{}), gate: make(chan struct{})}
	cw := catwalk.Model{ContextWindow: 1000, DefaultMaxTokens: 100}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  modelWith(m, "prov", "model", cw),
		SmallModel:  modelWith(&finishStreamModel{text: "title"}, "prov", "small", cw),
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
		Tools:       []fantasy.AgentTool{namedTool("noop")},
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	col := newRCCollector(t.Context(), broker)

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "work"})
		done <- err
	}()
	waitEntered(t, m.entered) // summary stream started
	_, err = sa.Run(bg(), SessionAgentCall{SessionID: sess.ID, RunID: "q", Prompt: "queued while summarizing"})
	require.NoError(t, err)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))
	sa.CancelRun(sess.ID, "x")
	select {
	case err := <-done:
		t.Logf("Run(x) returned err=%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run(x) did not return")
	}
	x := col.waitN(t, "x", 1, 3*time.Second)
	q := col.waitN(t, "q", 1, 3*time.Second)
	time.Sleep(100 * time.Millisecond)
	x, q = col.of("x"), col.of("q")
	t.Logf("x=%+v q=%+v modelCalls=%d state: %s", x, q, m.n.Load(), sa.probeInternals(sess.ID))
	require.Len(t, x, 1)
	require.True(t, x[0].Cancelled)
	require.Len(t, q, 1, "queued prompt must run after the summarize cancel")
	require.False(t, q[0].Cancelled)
	require.True(t, sa.probeClean(sess.ID), sa.probeInternals(sess.ID))
}

// Huỷ cả session lúc đang tóm tắt: bỏ hàng đợi và lượt tiếp-tục-sau-tóm-tắt.

func TestSessionCancelDuringSummarizeDropsQueue(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := &summarizeGateModel{entered: make(chan struct{}), gate: make(chan struct{})}
	cw := catwalk.Model{ContextWindow: 1000, DefaultMaxTokens: 100}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  modelWith(m, "prov", "model", cw),
		SmallModel:  modelWith(&finishStreamModel{text: "title"}, "prov", "small", cw),
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
		Tools:       []fantasy.AgentTool{namedTool("noop")},
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	col := newRCCollector(t.Context(), broker)
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sess.ID, RunID: "x", Prompt: "work"})
		done <- err
	}()
	waitEntered(t, m.entered)
	_, err = sa.Run(bg(), SessionAgentCall{SessionID: sess.ID, RunID: "q", Prompt: "queued while summarizing"})
	require.NoError(t, err)
	sa.Cancel(sess.ID)
	select {
	case err := <-done:
		t.Logf("Run(x) returned err=%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run(x) did not return")
	}
	col.waitN(t, "x", 1, 3*time.Second)
	col.waitN(t, "q", 1, 3*time.Second)
	time.Sleep(200 * time.Millisecond)
	x, q := col.of("x"), col.of("q")
	t.Logf("x=%+v q=%+v modelCalls=%d state: %s", x, q, m.n.Load(), sa.probeInternals(sess.ID))
	require.Len(t, x, 1)
	require.Len(t, q, 1)
	require.True(t, q[0].Cancelled, "Esc must drop the queued prompt")
	require.EqualValues(t, 2, m.n.Load(), "no model call after Esc")
	require.True(t, sa.probeClean(sess.ID), sa.probeInternals(sess.ID))
}

// Prompt xếp trước lần huỷ bị bỏ; prompt xếp sau lần huỷ (lúc lượt đang dừng) vẫn chạy.

type blockListService struct {
	message.Service
	armed   atomic.Bool
	entered chan struct{}
	gate    chan struct{}
	once    sync.Once
}

func (s *blockListService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	if s.armed.Load() {
		s.once.Do(func() {
			close(s.entered)
			<-s.gate
		})
	}
	return s.Service.List(ctx, sessionID)
}

func TestPromptQueuedAfterCancelSurvives(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := newProbeModel()
	m.gate("a")
	cw := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	bls := &blockListService{Service: env.messages, entered: make(chan struct{}), gate: make(chan struct{})}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: m, CatwalkCfg: cw},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: cw},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    bls,
		RunComplete: broker,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sid := sess.ID
	col := newRCCollector(t.Context(), broker)

	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "a", Prompt: "a"})
		done <- err
	}()
	waitEntered(t, m.enteredCh("a"))
	_, err = sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "q0", Prompt: "q0"})
	require.NoError(t, err)
	bls.armed.Store(true)
	sa.Cancel(sid) // Esc: cancels a, drops q0
	waitEntered(t, bls.entered)
	// a is now unwinding (still registered as active); a prompt sent after the Esc arrives.
	_, err = sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "p", Prompt: "p"})
	require.NoError(t, err)
	require.Equal(t, 1, sa.QueuedPrompts(sid))
	close(bls.gate)
	select {
	case err := <-done:
		t.Logf("Run(a) returned err=%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run(a) did not return")
	}
	for _, id := range []string{"a", "q0", "p"} {
		col.waitN(t, id, 1, 3*time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	t.Logf("a=%+v q0=%+v p=%+v prompts=%v", col.of("a"), col.of("q0"), col.of("p"), m.seen())
	require.Len(t, col.of("a"), 1)
	require.True(t, col.of("a")[0].Cancelled)
	require.Len(t, col.of("q0"), 1)
	require.True(t, col.of("q0")[0].Cancelled)
	require.Len(t, col.of("p"), 1)
	require.False(t, col.of("p")[0].Cancelled)
	require.Equal(t, []string{"a", "p"}, m.seen())
	require.True(t, sa.probeClean(sid), sa.probeInternals(sid))
}

// Bão ngẫu nhiên gửi / Cancel / CancelRun: không prompt nào chạy hai lần, mỗi RunID đúng một RunComplete.

func TestQueueCancelStorm(t *testing.T) {
	seeds := []uint64{}
	for i := uint64(1); i <= 5; i++ {
		seeds = append(seeds, i)
	}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) { probeStorm(t, seed) })
	}
}

func probeStorm(t *testing.T, seed uint64) {
	newRng := func(k uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed*7919+k)) }
	m := newProbeModel()
	m.delay = 8 * time.Millisecond
	var clock atomic.Int64
	var mu sync.Mutex
	submitAt := map[string]int64{}
	startAt := map[string]int64{}
	var cancels [][2]int64
	var lastStarted string
	m.onStart = func(prompt string, alive bool) {
		if !alive {
			return
		}
		mu.Lock()
		if _, dup := startAt[prompt]; !dup {
			startAt[prompt] = clock.Add(1)
		}
		lastStarted = prompt
		mu.Unlock()
	}
	sa, broker, sid, _ := probeAgent(t, m)
	col := newRCCollector(t.Context(), broker)

	const N = 80
	var wg sync.WaitGroup
	var runsWG sync.WaitGroup
	submit := func(i int) {
		id := fmt.Sprintf("r%03d", i)
		mu.Lock()
		submitAt[id] = clock.Add(1)
		mu.Unlock()
		acc := sa.BeginAccepted(sid)
		runsWG.Add(1)
		go func() {
			defer runsWG.Done()
			_, _ = sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: id, Prompt: id, Accepted: acc})
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		rng := newRng(1)
		for i := 0; i < N; i++ {
			submit(i)
			time.Sleep(time.Duration(2000+rng.IntN(9000)) * time.Microsecond)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		rng := newRng(2)
		for k := 0; k < 6; k++ {
			time.Sleep(time.Duration(60+rng.IntN(140)) * time.Millisecond)
			cb := clock.Add(1)
			sa.Cancel(sid)
			ce := clock.Add(1)
			mu.Lock()
			cancels = append(cancels, [2]int64{cb, ce})
			mu.Unlock()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		rng := newRng(3)
		for k := 0; k < 30; k++ {
			time.Sleep(time.Duration(10+rng.IntN(30)) * time.Millisecond)
			mu.Lock()
			cur := lastStarted
			mu.Unlock()
			if cur != "" && rng.IntN(2) == 0 {
				sa.CancelRun(sid, cur) // the active run
			} else {
				sa.CancelRun(sid, fmt.Sprintf("r%03d", rng.IntN(N)))
			}
		}
	}()
	wg.Wait()
	finished := make(chan struct{})
	go func() { runsWG.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(60 * time.Second):
		t.Fatalf("a Run call never returned (deadlock?) state: %s", sa.probeInternals(sid))
	}
	// quiesce
	dl := time.Now().Add(10 * time.Second)
	for time.Now().Before(dl) && !sa.probeClean(sid) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	t.Logf("final state: %s", sa.probeInternals(sid))
	{
		var started, cancelled, completed int
		for i := 0; i < N; i++ {
			id := fmt.Sprintf("r%03d", i)
			if got := col.of(id); len(got) == 1 {
				if got[0].Cancelled {
					cancelled++
				} else {
					completed++
				}
			}
		}
		mu.Lock()
		started = len(startAt)
		mu.Unlock()
		t.Logf("stats: started=%d completed=%d cancelled=%d", started, completed, cancelled)
	}

	var problems []string
	if !sa.probeClean(sid) {
		problems = append(problems, "not clean: "+sa.probeInternals(sid))
	}
	// I1: exactly one RunComplete per RunID
	for i := 0; i < N; i++ {
		id := fmt.Sprintf("r%03d", i)
		if got := col.of(id); len(got) != 1 {
			problems = append(problems, fmt.Sprintf("%s: %d RunCompletes", id, len(got)))
		}
	}
	// no prompt streamed twice
	m.mu.Lock()
	for p, c := range m.starts {
		if strings.HasPrefix(p, "r") && c > 1 {
			problems = append(problems, fmt.Sprintf("%s streamed %d times", p, c))
		}
	}
	m.mu.Unlock()
	// I3: nothing submitted before a Cancel starts after it completed
	mu.Lock()
	for id, st := range startAt {
		sub := submitAt[id]
		for _, c := range cancels {
			if sub < c[0] && st > c[1] {
				problems = append(problems, fmt.Sprintf("%s submitted@%d started@%d after Cancel[%d,%d]", id, sub, st, c[0], c[1]))
				break
			}
		}
	}
	mu.Unlock()
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatalf("%d problems:\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

// Khe chuyển tiếp (prompt đã lấy khỏi hàng đợi, Run chưa đăng ký) giữ đúng thứ tự.
func TestHandOffWindowKeepsOrder(t *testing.T) {
	m := newProbeModel()
	gA := m.gate("a")
	sa, _, sid, _ := probeAgent(t, m)
	hookIn := make(chan struct{})
	hookGate := make(chan struct{})
	var hookOnce sync.Once
	hook := func(rc notify.RunComplete) {
		hookOnce.Do(func() { close(hookIn); <-hookGate })
	}
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "a", Prompt: "a", OnComplete: hook})
		done <- err
	}()
	waitEntered(t, m.enteredCh("a"))
	for _, id := range []string{"q1", "q2"} {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: id, Prompt: id})
		require.NoError(t, err)
	}
	close(gA)
	select {
	case <-hookIn:
	case <-time.After(5 * time.Second):
		t.Fatal("hand-off never reached the OnComplete hook")
	}
	sa.CancelRun(sid, "f")
	fDone := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "f", Prompt: "f"})
		fDone <- err
	}()
	fFinished := false
	select {
	case <-fDone:
		fFinished = true
		t.Logf("f finished before the hook was released; prompts=%v", m.seen())
	case <-time.After(1500 * time.Millisecond):
		t.Logf("f still running 1.5s later; prompts=%v state=%s", m.seen(), sa.probeInternals(sid))
	}
	close(hookGate)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("run a did not finish")
	}
	if !fFinished {
		select {
		case <-fDone:
		case <-time.After(10 * time.Second):
			t.Fatal("run f did not finish")
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(m.seen()) < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	t.Logf("model saw prompts in order: %v; state: %s", m.seen(), sa.probeInternals(sid))
	require.Equal(t, []string{"a", "q1", "q2"}, m.seen())
	require.True(t, sa.probeClean(sid), sa.probeInternals(sid))
}

// Esc rơi vào khe chuyển tiếp: prompt vừa lấy ra không được chạy.
func TestEscInHandOffWindowDropsDequeuedPrompt(t *testing.T) {
	m := newProbeModel()
	gA := m.gate("a")
	sa, broker, sid, _ := probeAgent(t, m)
	col := newRCCollector(t.Context(), broker)
	hookIn := make(chan struct{})
	hookGate := make(chan struct{})
	var hookOnce sync.Once
	hook := func(rc notify.RunComplete) { hookOnce.Do(func() { close(hookIn); <-hookGate }) }
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: "a", Prompt: "a", OnComplete: hook})
		done <- err
	}()
	waitEntered(t, m.enteredCh("a"))
	for _, id := range []string{"q1", "q2"} {
		_, err := sa.Run(bg(), SessionAgentCall{SessionID: sid, RunID: id, Prompt: id})
		require.NoError(t, err)
	}
	close(gA)
	<-hookIn
	t.Logf("window: %s", sa.probeInternals(sid))
	sa.Cancel(sid) // Esc
	close(hookGate)
	<-done
	for _, id := range []string{"q1", "q2"} {
		col.waitN(t, id, 1, 3*time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	t.Logf("prompts run: %v; q1=%+v q2=%+v; state: %s", m.seen(), col.of("q1"), col.of("q2"), sa.probeInternals(sid))
	require.Equal(t, []string{"a"}, m.seen(), "a prompt ran after Esc")
	require.Len(t, col.of("q1"), 1)
	require.True(t, col.of("q1")[0].Cancelled)
	require.Len(t, col.of("q2"), 1)
	require.True(t, col.of("q2")[0].Cancelled)
	require.True(t, sa.probeClean(sid), sa.probeInternals(sid))
}

// CancelRun lượt có Stop hook: prompt xếp hàng chạy tiếp trong chuỗi của gate không bị đánh
// dấu Cancelled (chỉ lượt của gate, hoặc khi huỷ cả session).
func TestStopGateDoesNotCancelHandedOffPrompt(t *testing.T) {
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	m := newProbeModel()
	m.gate("first")
	cw := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: m, CatwalkCfg: cw},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: cw},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: holdRunComplete(broker),
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	col := newRCCollector(t.Context(), broker)
	stop, _ := stopHookScript(t, "exit 0\n")
	gated := WithStopHooks(probeCoordinator{sessionAgentCoordinator{sa: sa}}, stop)

	done := make(chan error, 1)
	go func() {
		_, err := gated.Run(WithRunID(bg(), "x"), sess.ID, "first")
		done <- err
	}()
	waitEntered(t, m.enteredCh("first"))
	res, err := gated.Run(WithRunID(bg(), "q"), sess.ID, "second")
	require.NoError(t, err)
	require.Nil(t, res)
	gated.CancelRun(sess.ID, "x")
	select {
	case err := <-done:
		t.Logf("gated Run(x) returned err=%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("gate did not return")
	}
	x := col.waitN(t, "x", 1, 3*time.Second)
	q := col.waitN(t, "q", 1, 3*time.Second)
	t.Logf("prompts run: %v\n x=%+v\n q=%+v", m.seen(), x, q)
	require.Len(t, q, 1)
	require.Equal(t, []string{"first", "second"}, m.seen(), "the queued prompt must have run")
	require.False(t, q[0].Cancelled, "q ran to completion but its waiter is told it was cancelled")
}

// probeCoordinator forwards the cancel / busy calls the shared test helper leaves nil.
type probeCoordinator struct{ sessionAgentCoordinator }

func (c probeCoordinator) Cancel(id string)             { c.sa.Cancel(id) }
func (c probeCoordinator) CancelRun(id, runID string)   { c.sa.CancelRun(id, runID) }
func (c probeCoordinator) CancelAll()                   { c.sa.CancelAll() }
func (c probeCoordinator) IsSessionBusy(id string) bool { return c.sa.IsSessionBusy(id) }
func (c probeCoordinator) IsBusy() bool                 { return c.sa.IsBusy() }

// ClearQueue (nút "clear queue") chạy đua với chuyển tiếp: không prompt nào vừa chạy vừa bị báo huỷ.

type countPub struct {
	mu sync.Mutex
	n  map[string]int
}

func (p *countPub) Publish(_ pubsub.EventType, rc notify.RunComplete) { p.add(rc) }
func (p *countPub) PublishMustDeliver(_ context.Context, _ pubsub.EventType, rc notify.RunComplete) {
	p.add(rc)
}
func (p *countPub) add(rc notify.RunComplete) {
	p.mu.Lock()
	p.n[rc.RunID]++
	p.mu.Unlock()
}

func TestClearQueueAgainstHandOff(t *testing.T) {
	env := testEnv(t)
	pub := &countPub{n: map[string]int{}}
	cw := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: newProbeModel(), CatwalkCfg: cw},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: cw},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: pub,
	}).(*sessionAgent)
	const sid = "race-session"
	var violations, resurrected int
	for i := 0; i < 20000; i++ {
		sa.messageQueue.Del(sid)
		pub.mu.Lock()
		pub.n = map[string]int{}
		pub.mu.Unlock()
		sa.enqueueCall(SessionAgentCall{SessionID: sid, RunID: "q1", Prompt: "q1"})
		sa.enqueueCall(SessionAgentCall{SessionID: sid, RunID: "q2", Prompt: "q2"})
		var next SessionAgentCall
		var ok bool
		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		go func() { defer wg.Done(); <-start; next, _, ok = sa.nextQueued(sid) }()
		go func() { defer wg.Done(); <-start; sa.ClearQueue(sid) }()
		close(start)
		wg.Wait()
		if ok {
			next.Accepted.Close()
		}
		pub.mu.Lock()
		cancelledQ1, cancelledQ2 := pub.n["q1"], pub.n["q2"]
		pub.mu.Unlock()
		left := sa.QueuedPrompts(sid)
		// q1 was handed out to run AND reported cancelled, or q2 reported cancelled yet still queued
		if ok && next.RunID == "q1" && cancelledQ1 > 0 {
			violations++
		}
		if left > 0 && (cancelledQ1+cancelledQ2) > 0 {
			resurrected++
		}
	}
	t.Logf("iterations=20000 handed-out-and-cancelled=%d cancelled-but-still-queued=%d", violations, resurrected)
	require.Zero(t, violations+resurrected)
}
