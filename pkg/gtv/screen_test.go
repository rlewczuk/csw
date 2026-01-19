package gtv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTRect_Contains(t *testing.T) {
	tests := []struct {
		name string
		rect TRect
		x    uint16
		y    uint16
		want bool
	}{
		{
			name: "point inside rectangle",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    15,
			y:    15,
			want: true,
		},
		{
			name: "point at top-left corner",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    10,
			y:    10,
			want: true,
		},
		{
			name: "point at bottom-right corner (exclusive)",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    30,
			y:    25,
			want: false,
		},
		{
			name: "point one before bottom-right corner",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    29,
			y:    24,
			want: true,
		},
		{
			name: "point outside left",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    9,
			y:    15,
			want: false,
		},
		{
			name: "point outside right",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    31,
			y:    15,
			want: false,
		},
		{
			name: "point outside top",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    15,
			y:    9,
			want: false,
		},
		{
			name: "point outside bottom",
			rect: TRect{X: 10, Y: 10, W: 20, H: 15},
			x:    15,
			y:    26,
			want: false,
		},
		{
			name: "zero-sized rectangle",
			rect: TRect{X: 10, Y: 10, W: 0, H: 0},
			x:    10,
			y:    10,
			want: false,
		},
		{
			name: "point at origin in origin rectangle",
			rect: TRect{X: 0, Y: 0, W: 80, H: 24},
			x:    0,
			y:    0,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rect.Contains(tt.x, tt.y)
			assert.Equal(t, tt.want, got, "TRect.Contains() at screen.go")
		})
	}
}

func TestTRect_RelativeTo(t *testing.T) {
	tests := []struct {
		name   string
		rect   TRect
		parent TRect
		want   TRect
	}{
		{
			name:   "child within parent",
			rect:   TRect{X: 5, Y: 5, W: 10, H: 8},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 15, Y: 15, W: 10, H: 8},
		},
		{
			name:   "child at parent origin",
			rect:   TRect{X: 0, Y: 0, W: 10, H: 8},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 10, Y: 10, W: 10, H: 8},
		},
		{
			name:   "child overflows parent width",
			rect:   TRect{X: 45, Y: 5, W: 20, H: 8},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 55, Y: 15, W: 5, H: 8},
		},
		{
			name:   "child overflows parent height",
			rect:   TRect{X: 5, Y: 35, W: 10, H: 20},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 15, Y: 45, W: 10, H: 5},
		},
		{
			name:   "child overflows both dimensions",
			rect:   TRect{X: 45, Y: 35, W: 20, H: 20},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 55, Y: 45, W: 5, H: 5},
		},
		{
			name:   "child completely outside parent (right)",
			rect:   TRect{X: 55, Y: 5, W: 10, H: 8},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 65, Y: 15, W: 0, H: 8},
		},
		{
			name:   "child completely outside parent (bottom)",
			rect:   TRect{X: 5, Y: 45, W: 10, H: 8},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 15, Y: 55, W: 10, H: 0},
		},
		{
			name:   "parent at screen origin",
			rect:   TRect{X: 5, Y: 5, W: 10, H: 8},
			parent: TRect{X: 0, Y: 0, W: 80, H: 24},
			want:   TRect{X: 5, Y: 5, W: 10, H: 8},
		},
		{
			name:   "child same size as parent",
			rect:   TRect{X: 0, Y: 0, W: 50, H: 40},
			parent: TRect{X: 10, Y: 10, W: 50, H: 40},
			want:   TRect{X: 10, Y: 10, W: 50, H: 40},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rect.RelativeTo(tt.parent)
			assert.Equal(t, tt.want, got, "TRect.RelativeTo() at screen.go")
		})
	}
}
