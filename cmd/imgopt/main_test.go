package main

import (
	"image"
	"image/color"
	"testing"
)

// newTestImage returns a w×h image filled with a non-uniform pattern so a
// resampler has something real to work on.
func newTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	return img
}

// resizeToWidth must scale to the requested width and derive the height from
// the source aspect ratio — matching the old imaging.Resize(w, 0) contract.
func TestResizeToWidth_PreservesAspectRatio(t *testing.T) {
	src := newTestImage(200, 100) // 2:1
	got := resizeToWidth(src, 50)
	if b := got.Bounds(); b.Dx() != 50 || b.Dy() != 25 {
		t.Errorf("resizeToWidth(200x100, 50) = %dx%d, want 50x25", b.Dx(), b.Dy())
	}
}

// A target width that would round the height below 1px must still yield a
// valid (≥1px tall) image rather than an empty/zero-height one.
func TestResizeToWidth_ClampsTinyHeight(t *testing.T) {
	src := newTestImage(1000, 1) // extreme aspect ratio
	got := resizeToWidth(src, 2)
	if b := got.Bounds(); b.Dy() < 1 {
		t.Errorf("height clamped to %d, want ≥1", b.Dy())
	}
}

// Non-positive widths and degenerate sources are passed through untouched, so
// the caller's `if width > 0` guard and any zero-area input stay safe.
func TestResizeToWidth_PassThrough(t *testing.T) {
	src := newTestImage(80, 40)
	if got := resizeToWidth(src, 0); got != src {
		t.Error("width 0 should return the source image unchanged")
	}
	if got := resizeToWidth(src, -10); got != src {
		t.Error("negative width should return the source image unchanged")
	}

	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if got := resizeToWidth(empty, 100); got != image.Image(empty) {
		t.Error("zero-area source should return unchanged")
	}
}
