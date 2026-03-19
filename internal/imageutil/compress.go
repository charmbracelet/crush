package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"

	"github.com/disintegration/imaging"
)

// CompressionConfig holds the configuration for image compression.
type CompressionConfig struct {
	// MaxSizeBytes is the maximum size in bytes before compression is triggered.
	// Default is 1MB.
	MaxSizeBytes int64

	// JPEGQuality is the quality for JPEG compression (1-100).
	// Default is 75 which provides good quality with smaller size.
	JPEGQuality int

	// MaxDimension is the maximum width or height for the image.
	// Images larger than this will be resized proportionally.
	// Default is 2048 pixels.
	MaxDimension int
}

// DefaultCompressionConfig returns the default compression configuration.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MaxSizeBytes: 1024 * 1024, // 1MB
		JPEGQuality:  75,
		MaxDimension: 2048,
	}
}

// CompressResult contains the compressed image data and metadata.
type CompressResult struct {
	Data     []byte
	MimeType string
	WasCompressed bool
	OriginalSize int64
	CompressedSize int64
}

// DetectMimeType detects the MIME type from image data.
func DetectMimeType(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// Check for PNG.
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// Check for JPEG.
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// Check for GIF.
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// Check for WebP.
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	return ""
}

// ShouldCompress checks if the image data should be compressed based on size.
func ShouldCompress(data []byte, config CompressionConfig) bool {
	return int64(len(data)) > config.MaxSizeBytes
}

// CompressImage compresses the image if it exceeds the size threshold.
// It converts PNG/GIF/WebP to JPEG for better compression when the image
// exceeds the size threshold. For images with transparency, it preserves
// PNG format.
func CompressImage(data []byte, mimeType string, config CompressionConfig) (*CompressResult, error) {
	originalSize := int64(len(data))

	// If under threshold, return original.
	if !ShouldCompress(data, config) {
		return &CompressResult{
			Data:          data,
			MimeType:      mimeType,
			WasCompressed: false,
			OriginalSize:  originalSize,
			CompressedSize: originalSize,
		}, nil
	}

	// Decode the image.
	var img image.Image
	var err error

	reader := bytes.NewReader(data)

	switch mimeType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	default:
		// Try generic decoding.
		img, err = imaging.Decode(reader)
	}

	if err != nil {
		slog.Warn("Failed to decode image for compression, returning original", "error", err, "mime_type", mimeType)
		return &CompressResult{
			Data:          data,
			MimeType:      mimeType,
			WasCompressed: false,
			OriginalSize:  originalSize,
			CompressedSize: originalSize,
		}, nil
	}

	// Resize if needed.
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > config.MaxDimension || height > config.MaxDimension {
		img = imaging.Fit(img, config.MaxDimension, config.MaxDimension, imaging.Lanczos)
	}

	// Check if image has transparency.
	hasTransparency := hasAlpha(img)

	var output bytes.Buffer
	var outputMimeType string

	if hasTransparency {
		// Keep PNG for images with transparency.
		outputMimeType = "image/png"
		if err := png.Encode(&output, img); err != nil {
			slog.Warn("Failed to encode PNG, returning original", "error", err)
			return &CompressResult{
				Data:          data,
				MimeType:      mimeType,
				WasCompressed: false,
				OriginalSize:  originalSize,
				CompressedSize: originalSize,
			}, nil
		}
	} else {
		// Convert to JPEG for better compression.
		outputMimeType = "image/jpeg"
		if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: config.JPEGQuality}); err != nil {
			slog.Warn("Failed to encode JPEG, returning original", "error", err)
			return &CompressResult{
				Data:          data,
				MimeType:      mimeType,
				WasCompressed: false,
				OriginalSize:  originalSize,
				CompressedSize: originalSize,
			}, nil
		}
	}

	compressedData := output.Bytes()
	compressedSize := int64(len(compressedData))

	// Only use compressed data if it's actually smaller than original.
	if compressedSize >= originalSize {
		slog.Debug("Compression did not reduce size, keeping original",
			"original_size", originalSize,
			"compressed_size", compressedSize,
		)
		return &CompressResult{
			Data:          data,
			MimeType:      mimeType,
			WasCompressed: false,
			OriginalSize:  originalSize,
			CompressedSize: originalSize,
		}, nil
	}

	slog.Debug("Image compressed",
		"original_size", originalSize,
		"compressed_size", compressedSize,
		"ratio", float64(compressedSize)/float64(originalSize),
		"output_format", outputMimeType,
	)

	return &CompressResult{
		Data:          compressedData,
		MimeType:      outputMimeType,
		WasCompressed: true,
		OriginalSize:  originalSize,
		CompressedSize: compressedSize,
	}, nil
}

// hasAlpha checks if the image has an alpha channel with non-opaque pixels.
func hasAlpha(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xFFFF {
				return true
			}
		}
	}
	return false
}

// CompressFromReader reads image data from a reader and compresses it if needed.
func CompressFromReader(r io.Reader, mimeType string, config CompressionConfig) (*CompressResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Auto-detect MIME type if not provided.
	if mimeType == "" {
		mimeType = DetectMimeType(data)
	}

	return CompressImage(data, mimeType, config)
}
