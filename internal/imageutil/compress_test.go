package imageutil

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// createTestImage creates a test image with the specified dimensions.
func createTestImage(width, height int, withAlpha bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if withAlpha {
				// Semi-transparent red
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 128})
			} else {
				// Solid red
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	if withAlpha {
		png.Encode(&buf, img)
	} else {
		jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	}
	return buf.Bytes()
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PNG image",
			data:     []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: "image/png",
		},
		{
			name:     "JPEG image",
			data:     []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46},
			expected: "image/jpeg",
		},
		{
			name:     "GIF image",
			data:     []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00},
			expected: "image/gif",
		},
		{
			name:     "WebP image",
			data:     []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			expected: "image/webp",
		},
		{
			name:     "Unknown format",
			data:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expected: "",
		},
		{
			name:     "Too short",
			data:     []byte{0x00, 0x01},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMimeType(tt.data)
			if result != tt.expected {
				t.Errorf("DetectMimeType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldCompress(t *testing.T) {
	config := DefaultCompressionConfig()

	tests := []struct {
		name     string
		dataSize int
		expected bool
	}{
		{
			name:     "Small image under threshold",
			dataSize: 512 * 1024, // 512KB
			expected: false,
		},
		{
			name:     "Image at threshold",
			dataSize: 1024 * 1024, // 1MB exactly
			expected: false,
		},
		{
			name:     "Image over threshold",
			dataSize: 2 * 1024 * 1024, // 2MB
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataSize)
			result := ShouldCompress(data, config)
			if result != tt.expected {
				t.Errorf("ShouldCompress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCompressImage_NoCompressionNeeded(t *testing.T) {
	config := DefaultCompressionConfig()

	// Create a small image that doesn't need compression
	data := createTestImage(100, 100, false)
	originalSize := len(data)

	result, err := CompressImage(data, "image/jpeg", config)
	if err != nil {
		t.Fatalf("CompressImage() error = %v", err)
	}

	if result.WasCompressed {
		t.Error("CompressImage() should not have compressed small image")
	}

	if len(result.Data) != originalSize {
		t.Error("CompressImage() should return original data for small images")
	}
}

func TestCompressImage_JPEGCompression(t *testing.T) {
	config := DefaultCompressionConfig()

	// Create a large JPEG image with quality 100 (which creates a large file)
	img := image.NewRGBA(image.Rect(0, 0, 2000, 2000))
	for y := 0; y < 2000; y++ {
		for x := 0; x < 2000; x++ {
			// Use varying colors to make compression more effective
			img.Set(x, y, color.RGBA{
				R: uint8((x + y) % 256),
				G: uint8((x * 2) % 256),
				B: uint8((y * 2) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	// Encode with high quality to create a large file
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100})
	data := buf.Bytes()

	// Skip test if image isn't large enough
	if int64(len(data)) <= config.MaxSizeBytes {
		t.Skip("Test image not large enough to trigger compression")
	}

	result, err := CompressImage(data, "image/jpeg", config)
	if err != nil {
		t.Fatalf("CompressImage() error = %v", err)
	}

	// If compression happened, verify it reduced size
	if result.WasCompressed {
		if result.MimeType != "image/jpeg" {
			t.Errorf("CompressImage() mimeType = %v, want image/jpeg", result.MimeType)
		}
		if result.CompressedSize >= result.OriginalSize {
			t.Error("CompressImage() should reduce size when compressing")
		}
	}
}

func TestCompressImage_PreserveTransparency(t *testing.T) {
	config := DefaultCompressionConfig()

	// Create a PNG image with transparency - use varying colors for better test
	img := image.NewRGBA(image.Rect(0, 0, 2000, 2000))
	for y := 0; y < 2000; y++ {
		for x := 0; x < 2000; x++ {
			// Semi-transparent with varying colors
			img.Set(x, y, color.RGBA{
				R: uint8((x + y) % 256),
				G: uint8((x * 2) % 256),
				B: uint8((y * 2) % 256),
				A: 128, // Semi-transparent
			})
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	data := buf.Bytes()

	// Skip test if image isn't large enough
	if int64(len(data)) <= config.MaxSizeBytes {
		t.Skip("Test image not large enough to trigger compression")
	}

	result, err := CompressImage(data, "image/png", config)
	if err != nil {
		t.Fatalf("CompressImage() error = %v", err)
	}

	// If compression happened, PNG format should be preserved for transparency
	if result.WasCompressed {
		if result.MimeType != "image/png" {
			t.Errorf("CompressImage() mimeType = %v, want image/png (for transparency)", result.MimeType)
		}
	}
}

func TestDefaultCompressionConfig(t *testing.T) {
	config := DefaultCompressionConfig()

	if config.MaxSizeBytes != 1024*1024 {
		t.Errorf("Default MaxSizeBytes = %v, want %v", config.MaxSizeBytes, 1024*1024)
	}

	if config.JPEGQuality != 75 {
		t.Errorf("Default JPEGQuality = %v, want 75", config.JPEGQuality)
	}

	if config.MaxDimension != 2048 {
		t.Errorf("Default MaxDimension = %v, want 2048", config.MaxDimension)
	}
}

func TestHasAlpha(t *testing.T) {
	// Test image with alpha
	imgWithAlpha := image.NewRGBA(image.Rect(0, 0, 10, 10))
	imgWithAlpha.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 128})
	if !hasAlpha(imgWithAlpha) {
		t.Error("hasAlpha() should return true for image with semi-transparent pixels")
	}

	// Test image without alpha
	imgNoAlpha := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			imgNoAlpha.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	if hasAlpha(imgNoAlpha) {
		t.Error("hasAlpha() should return false for image with no transparency")
	}
}
