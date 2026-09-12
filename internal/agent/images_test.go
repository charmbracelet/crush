package agent

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// testPNG builds a noisy PNG; noise is incompressible, so sizes stay real.
func testPNG(t testing.TB, w, h int, seed int64) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(seed))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(r.Intn(256)), uint8(r.Intn(256)), uint8(r.Intn(256)), 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// testPhotoPNG builds a photograph-like image: PNG stores it poorly, JPEG well.
func testPhotoPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(7))
	jitter := func(base int) uint8 {
		return uint8(min(255, max(0, base+r.Intn(12)-6)))
	}
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				jitter(x * 255 / w),
				jitter(y * 255 / h),
				jitter((x + y) * 255 / (w + h)),
				255,
			})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// testScreenshotPNG builds a screenshot-like image: flat blocks and rules,
// which PNG stores better than JPEG.
func testScreenshotPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{24, 24, 32, 255}
	for y := range h {
		for x := range w {
			img.Set(x, y, bg)
		}
	}
	panel := color.RGBA{200, 200, 210, 255}
	for row := 0; row < h-40; row += 40 {
		for y := row; y < row+14 && y < h; y++ {
			for x := 20; x < w-20; x++ {
				img.Set(x, y, panel)
			}
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func dimsOf(t testing.TB, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	return cfg.Width, cfg.Height
}

func imageMessages(t testing.TB, n int, data []byte) []fantasy.Message {
	t.Helper()
	msgs := make([]fantasy.Message, 0, n)
	for range n {
		msgs = append(msgs, fantasy.NewUserMessage("look", fantasy.FilePart{
			Data: data, MediaType: "image/png", Filename: "shot.png",
		}))
	}
	return msgs
}

func firstFilePart(t testing.TB, msg fantasy.Message) fantasy.FilePart {
	t.Helper()
	for _, part := range msg.Content {
		if fp, ok := part.(fantasy.FilePart); ok {
			return fp
		}
	}
	t.Fatal("no file part in message")
	return fantasy.FilePart{}
}

func TestBoundImagesLeavesSmallRequestsAlone(t *testing.T) {
	t.Parallel()

	// Over the many-image cap but under the 8000px otherwise allowed.
	big := testPNG(t, 3000, 1000, 1)
	msgs := imageMessages(t, manyImageThreshold, big)

	got := boundImages(msgs, "anthropic")
	w, _ := dimsOf(t, firstFilePart(t, got[0]).Data)
	require.Equal(t, 3000, w, "at the threshold the stricter cap does not apply yet")
}

func TestBoundImagesShrinksOnceOverTheThreshold(t *testing.T) {
	t.Parallel()

	big := testPNG(t, 3000, 1500, 2)
	msgs := imageMessages(t, manyImageThreshold+1, big)

	got := boundImages(msgs, "anthropic")
	require.Len(t, got, len(msgs))
	for i := range got {
		w, h := dimsOf(t, firstFilePart(t, got[i]).Data)
		require.LessOrEqual(t, w, manyImageMaxDim)
		require.LessOrEqual(t, h, manyImageMaxDim)
		// 3000x1500 fits as 2000x1000: aspect ratio preserved.
		require.Equal(t, 2000, w)
		require.Equal(t, 1000, h)
	}
}

func TestBoundImagesLeavesInSpecImagesUntouched(t *testing.T) {
	t.Parallel()

	small := testPNG(t, 1920, 1080, 3)
	msgs := imageMessages(t, manyImageThreshold+5, small)

	got := boundImages(msgs, "anthropic")
	require.Equal(t, small, firstFilePart(t, got[0]).Data,
		"an image already within the cap must be passed through byte for byte")
}

func TestBoundImagesCountsAndShrinksToolResults(t *testing.T) {
	t.Parallel()

	big := testPNG(t, 2400, 2400, 4)
	encoded := base64.StdEncoding.EncodeToString(big)

	msgs := make([]fantasy.Message, 0, manyImageThreshold+1)
	for range manyImageThreshold + 1 {
		msgs = append(msgs, fantasy.Message{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{fantasy.ToolResultPart{
				ToolCallID: "call-1",
				Output: fantasy.ToolResultOutputContentMedia{
					Data: encoded, MediaType: "image/png",
				},
			}},
		})
	}

	require.Greater(t, countImageBlocks(msgs), manyImageThreshold,
		"images nested in tool results count toward the threshold")

	got := boundImages(msgs, "anthropic")
	part, ok := got[0].Content[0].(fantasy.ToolResultPart)
	require.True(t, ok)
	media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](part.Output)
	require.True(t, ok)
	decoded, err := base64.StdEncoding.DecodeString(media.Data)
	require.NoError(t, err)
	w, h := dimsOf(t, decoded)
	require.Equal(t, 2000, w)
	require.Equal(t, 2000, h)
}

func TestBoundImagesPassesThroughUndecodableData(t *testing.T) {
	t.Parallel()

	// Undecodable data is forwarded, not dropped.
	junk := []byte("not actually an image")
	msgs := imageMessages(t, manyImageThreshold+1, junk)

	got := boundImages(msgs, "anthropic")
	require.Equal(t, junk, firstFilePart(t, got[0]).Data)
}

func TestBoundImagesDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	big := testPNG(t, 3000, 1500, 5)
	msgs := imageMessages(t, manyImageThreshold+1, big)

	_ = boundImages(msgs, "anthropic")
	require.Equal(t, big, firstFilePart(t, msgs[0]).Data,
		"the caller's messages must be left alone; PrepareStep reuses them")
}

func TestServesAnthropicModels(t *testing.T) {
	t.Parallel()

	// The cap follows Anthropic's models, not inline-media support.
	for _, p := range []string{"anthropic", "bedrock", "bedrock-europe"} {
		require.True(t, servesAnthropicModels(p), "%s serves Anthropic's models", p)
	}

	// vertexai routes to Gemini, so Anthropic's limits do not apply.
	for _, p := range []string{"vertexai", "gemini", "openai", "openrouter", "azure", ""} {
		require.False(t, servesAnthropicModels(p), "%s does not serve Anthropic's models", p)
	}
}

func TestBudgetFor(t *testing.T) {
	t.Parallel()

	// Bytes are always budgeted; dimensions only past the threshold.
	under := budgetFor("anthropic", manyImageThreshold)
	require.Equal(t, 0, under.maxDim, "dimensions are uncapped below the threshold")
	require.Equal(t, maxImageBytesDirect, under.maxBytes)

	over := budgetFor("anthropic", manyImageThreshold+1)
	require.Equal(t, manyImageMaxDim, over.maxDim)

	// Bedrock allows half the payload.
	for _, p := range []string{"bedrock", "bedrock-europe"} {
		require.Equal(t, maxImageBytesPartner, budgetFor(p, 1).maxBytes, p)
	}
}

func TestFitImageCapsBytesWithoutMovingPixels(t *testing.T) {
	t.Parallel()

	// With no dimension cap, bytes must come down by re-encoding alone:
	// the model's coordinates are relative to the image it was shown.
	src := testPhotoPNG(t, 1500, 1200)
	budget := imageBudget{maxBytes: 512 << 10}
	require.Greater(t, base64Len(len(src)), budget.maxBytes, "test image must start over budget")

	out, mediaType, changed := fitImage(src, "image/png", budget)
	require.True(t, changed)
	require.LessOrEqual(t, base64Len(len(out)), budget.maxBytes, "must land inside the byte budget")
	require.Equal(t, "image/jpeg", mediaType, "re-encoding is how bytes come down here")

	w, h := dimsOf(t, out)
	require.Equal(t, 1500, w, "width must survive a byte-only refit")
	require.Equal(t, 1200, h, "height must survive a byte-only refit")
}

func TestFitImageLeavesInBudgetImagesAlone(t *testing.T) {
	t.Parallel()

	src := testPNG(t, 40, 40, 12)
	_, _, changed := fitImage(src, "image/png", imageBudget{maxBytes: maxImageBytesDirect})
	require.False(t, changed, "an image inside both budgets must not be touched")
}

func TestFitImageAppliesBothBudgets(t *testing.T) {
	t.Parallel()

	// 4000x4000 scaled to 2000x2000 still encodes to ~15MB.
	src := testPNG(t, 4000, 4000, 13)
	budget := imageBudget{maxDim: manyImageMaxDim, maxBytes: maxImageBytesDirect}

	out, _, changed := fitImage(src, "image/png", budget)
	require.True(t, changed)

	w, h := dimsOf(t, out)
	require.LessOrEqual(t, w, manyImageMaxDim)
	require.LessOrEqual(t, h, manyImageMaxDim)
	require.LessOrEqual(t, base64Len(len(out)), budget.maxBytes,
		"bounding dimensions alone does not bound bytes")
}

func TestEncodeWithinBudgetKeepsScreenshotsLossless(t *testing.T) {
	t.Parallel()

	// JPEG is ~3x the size of PNG here, and lossy. PNG must win.
	src := testScreenshotPNG(t, 1500, 1200)
	img, _, err := image.Decode(bytes.NewReader(src))
	require.NoError(t, err)

	out, mediaType, err := encodeWithinBudget(img, "image/png", maxImageBytesDirect)
	require.NoError(t, err)
	require.Equal(t, "image/png", mediaType, "a screenshot that fits as PNG must not be degraded to JPEG")
	require.LessOrEqual(t, base64Len(len(out)), maxImageBytesDirect)
}

func TestEncodeWithinBudgetScalesOnlyAsALastResort(t *testing.T) {
	t.Parallel()

	// Incompressible noise: no quality step reaches the budget.
	img, _, err := image.Decode(bytes.NewReader(testPNG(t, 1200, 1200, 21)))
	require.NoError(t, err)

	out, _, err := encodeWithinBudget(img, "image/png", 16<<10)
	require.NoError(t, err)
	w, h := dimsOf(t, out)
	require.Less(t, w, 1200, "scaling is the last resort, but it must happen")
	require.Equal(t, w, h, "aspect ratio survives scaling")
}
