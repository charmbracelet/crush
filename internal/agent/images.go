package agent

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif" // registered so GIF dimensions can be read
	"image/jpeg"
	"image/png"
	"log/slog"
	"strings"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/disintegration/imaging"
	"github.com/zeebo/xxh3"
)

// Past 20 image blocks in one request, Anthropic caps every image in it at
// 2000px and rejects oversized ones instead of downscaling.
const (
	manyImageThreshold = 20
	manyImageMaxDim    = 2000
)

// Per-image base64 budget. Capping dimensions does not cap bytes: 2000x2000
// can still encode to ~15MB.
const (
	maxImageBytesDirect  = 10 << 20
	maxImageBytesPartner = 5 << 20
)

// imageBudget is what one image must fit inside. maxDim of zero leaves
// dimensions alone, which is the usual case.
type imageBudget struct {
	maxDim   int
	maxBytes int
}

// budgetFor returns the budget for each image in a request of this size.
func budgetFor(provider string, imageCount int) imageBudget {
	b := imageBudget{maxBytes: maxImageBytesDirect}
	switch provider {
	case string(catwalk.InferenceProviderBedrock), string(catwalk.InferenceProviderBedrockEurope):
		b.maxBytes = maxImageBytesPartner
	}
	if imageCount > manyImageThreshold {
		b.maxDim = manyImageMaxDim
	}
	return b
}

// base64Len is the encoded length of n bytes; the limits count encoded size.
func base64Len(n int) int { return (n + 2) / 3 * 4 }

// boundImages refits every image in a request that exceeds its budget.
// Images already inside it are returned untouched.
func boundImages(messages []fantasy.Message, provider string) []fantasy.Message {
	budget := budgetFor(provider, countImageBlocks(messages))

	out := make([]fantasy.Message, len(messages))
	copy(out, messages)
	for i := range out {
		var content []fantasy.MessagePart
		for j, part := range out[i].Content {
			bounded, ok := boundPart(part, budget)
			if !ok {
				continue
			}
			if content == nil {
				content = make([]fantasy.MessagePart, len(out[i].Content))
				copy(content, out[i].Content)
			}
			content[j] = bounded
		}
		if content != nil {
			out[i].Content = content
		}
	}
	return out
}

// boundPart returns a refitted copy of part, and whether it changed.
func boundPart(part fantasy.MessagePart, budget imageBudget) (fantasy.MessagePart, bool) {
	switch p := part.(type) {
	case fantasy.FilePart:
		data, mediaType, ok := fitImage(p.Data, p.MediaType, budget)
		if !ok {
			return nil, false
		}
		p.Data = data
		p.MediaType = mediaType
		return p, true
	case fantasy.ToolResultPart:
		media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](p.Output)
		if !ok {
			return nil, false
		}
		decoded, err := base64Decode(media.Data)
		if err != nil {
			return nil, false
		}
		data, mediaType, ok := fitImage(decoded, media.MediaType, budget)
		if !ok {
			return nil, false
		}
		media.Data = base64Encode(data)
		media.MediaType = mediaType
		p.Output = media
		return p, true
	}
	return nil, false
}

// countImageBlocks counts image blocks, including those inside tool results.
func countImageBlocks(messages []fantasy.Message) int {
	var n int
	for _, msg := range messages {
		for _, part := range msg.Content {
			switch p := part.(type) {
			case fantasy.FilePart:
				if isImage(p.MediaType) {
					n++
				}
			case fantasy.ToolResultPart:
				if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](p.Output); ok && isImage(media.MediaType) {
					n++
				}
			}
		}
	}
	return n
}

// imageKey identifies a cached refit by content and budget.
func imageKey(data []byte, budget imageBudget) [16]byte {
	h := xxh3.Hash128(data)
	var key [16]byte
	// The budget is part of the key: the same bytes refit differently under
	// different limits.
	binary.BigEndian.PutUint64(key[:8], h.Hi^uint64(budget.maxDim))
	binary.BigEndian.PutUint64(key[8:], h.Lo^uint64(budget.maxBytes))
	return key
}

func base64Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

func base64Encode(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func isImage(mediaType string) bool {
	return strings.HasPrefix(mediaType, "image/")
}

// Refitting costs a few hundred milliseconds and PrepareStep runs per step,
// so results are cached. On overflow the cache is dropped whole rather than
// evicted by recency; a session works with few images at a time.
const shrinkCacheBudget = 64 << 20

type shrunkImage struct {
	data      []byte
	mediaType string
}

var shrinkCache = struct {
	sync.Mutex
	entries map[[16]byte]shrunkImage
	bytes   int
}{entries: map[[16]byte]shrunkImage{}}

func cachedShrink(key [16]byte) (shrunkImage, bool) {
	shrinkCache.Lock()
	defer shrinkCache.Unlock()
	got, ok := shrinkCache.entries[key]
	return got, ok
}

func storeShrink(key [16]byte, val shrunkImage) {
	shrinkCache.Lock()
	defer shrinkCache.Unlock()
	if shrinkCache.bytes+len(val.data) > shrinkCacheBudget {
		shrinkCache.entries = map[[16]byte]shrunkImage{}
		shrinkCache.bytes = 0
	}
	shrinkCache.entries[key] = val
	shrinkCache.bytes += len(val.data)
}

// fitImage brings data inside budget, returning the replacement and its
// media type, or false if nothing was needed or the image could not be read.
func fitImage(data []byte, mediaType string, budget imageBudget) ([]byte, string, bool) {
	if !isImage(mediaType) {
		return nil, "", false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		slog.Debug("Could not read image dimensions, sending unchanged", "media_type", mediaType, "error", err)
		return nil, "", false
	}

	oversized := budget.maxDim > 0 && (cfg.Width > budget.maxDim || cfg.Height > budget.maxDim)
	overweight := base64Len(len(data)) > budget.maxBytes
	if !oversized && !overweight {
		return nil, "", false
	}

	key := imageKey(data, budget)
	if hit, ok := cachedShrink(key); ok {
		return hit.data, hit.mediaType, true
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		slog.Debug("Could not decode image, sending unchanged", "media_type", mediaType, "error", err)
		return nil, "", false
	}
	if oversized {
		img = imaging.Fit(img, budget.maxDim, budget.maxDim, imaging.Lanczos)
	}

	encoded, outType, err := encodeWithinBudget(img, mediaType, budget.maxBytes)
	if err != nil {
		slog.Debug("Could not re-encode image, sending unchanged", "media_type", mediaType, "error", err)
		return nil, "", false
	}

	storeShrink(key, shrunkImage{data: encoded, mediaType: outType})
	slog.Debug("Refit image for Anthropic's per-image limits",
		"from", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"to", fmt.Sprintf("%dx%d", img.Bounds().Dx(), img.Bounds().Dy()),
		"bytes_before", len(data), "bytes_after", len(encoded), "media_type", outType)
	return encoded, outType, true
}

// Tried in order when an image is too heavy to send losslessly.
var jpegQualitySteps = []int{90, 75, 60}

// encodeWithinBudget encodes img to fit maxBytes, smallest format that fits.
// Both PNG and JPEG are tried at each size: PNG wins on screenshots, JPEG on
// photographs, by an order of magnitude either way. Scaling comes last, since
// it moves the coordinates the model reports.
func encodeWithinBudget(img image.Image, _ string, maxBytes int) ([]byte, string, error) {
	var best []byte
	var bestType string

	// Keeps the smallest candidate seen; reports whether it fits.
	consider := func(data []byte, mediaType string) bool {
		if best == nil || len(data) < len(best) {
			best, bestType = data, mediaType
		}
		return base64Len(len(data)) <= maxBytes
	}

	fits := func(im image.Image) (bool, error) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, im); err != nil {
			return false, err
		}
		if consider(buf.Bytes(), "image/png") {
			return true, nil
		}
		for _, q := range jpegQualitySteps {
			var jbuf bytes.Buffer
			if err := jpeg.Encode(&jbuf, im, &jpeg.Options{Quality: q}); err != nil {
				return false, err
			}
			if consider(jbuf.Bytes(), "image/jpeg") {
				return true, nil
			}
		}
		return false, nil
	}

	ok, err := fits(img)
	if err != nil {
		return nil, "", err
	}
	if ok {
		return best, bestType, nil
	}

	// Nothing fits at full size. Halve until it does.
	scaled := img
	for range 4 {
		w, h := scaled.Bounds().Dx()/2, scaled.Bounds().Dy()/2
		if w < 64 || h < 64 {
			break
		}
		scaled = imaging.Resize(scaled, w, h, imaging.Lanczos)
		ok, err := fits(scaled)
		if err != nil {
			return nil, "", err
		}
		if ok {
			slog.Debug("Scaled image down to fit the per-image byte budget",
				"to", fmt.Sprintf("%dx%d", w, h))
			return best, bestType, nil
		}
	}

	// Send the smallest we managed rather than nothing.
	slog.Warn("Could not fit image inside the per-image byte budget",
		"bytes", len(best), "budget", maxBytes)
	return best, bestType, nil
}

// servesAnthropicModels reports whether a provider enforces Anthropic's
// request limits. Vertex AI is excluded: Crush routes it to Gemini.
func servesAnthropicModels(provider string) bool {
	switch provider {
	case string(catwalk.InferenceProviderAnthropic),
		string(catwalk.InferenceProviderBedrock),
		string(catwalk.InferenceProviderBedrockEurope):
		return true
	default:
		return false
	}
}
