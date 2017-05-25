package main

import (
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"testing"
)

func TestCoordTransform(t *testing.T) {
	r := image.Rect(10, 20, 50, 60)
	if offset(r, 10, 20) != 0 {
		t.Fatal("coordinate transform error")
	}
	if offset(r, 49, 59) != r.Dx()*r.Dy()-1 {
		t.Fatal("coordinate transform error")
	}
	x, y := coords(r, offset(r, 15, 26))
	if x != 15 || y != 26 {
		t.Fatal("coordinate transform error")
	}
}

func randomImage(w, h int) (r *image.Gray) {
	r = image.NewGray(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			r.Pix[r.PixOffset(x, y)] = uint8(rand.Intn(256))
		}
	}
	return
}

func TestFindHits(t *testing.T) {
	r := image.Rect(0, 0, 10, 10)
	d := make([]float64, 100)
	for i := range d {
		d[i] = 1
	}
	d[offset(r, 2, 2)] = 0.1
	h := findHits(d, 0, 2, 0.3, 10, r)
	if len(h) != 1 || h[0] != (Hit{2, 2, 0.05}) {
		t.Fatal("findHits error")
	}
}

// generte random field and object images, place the object in the field,
// and test that objSearch can find it
func TestObjSearch(t *testing.T) {
	field := randomImage(100, 100)
	object := randomImage(10, 10)
	// obscured object
	draw.Draw(field, object.Bounds().Add(image.Point{20, 30}), object, image.ZP, draw.Src)
	// exact object (obscured above)
	draw.Draw(field, object.Bounds().Add(image.Point{26, 36}), object, image.ZP, draw.Src)
	r, _ := os.Create("test.png")
	png.Encode(r, field)
	r.Close()
	d, _, max := objSearch(field, object, field.Bounds())
	h := findHits(d, 0, max, 0.2, 0, field.Bounds())
	if len(h) != 2 || h[0] != (Hit{26, 36, 0}) || h[1].X != 20 || h[1].Y != 30 {
		// expect exact hit at 26,36 and approx hit at 20,30, and no others
		t.Error(h)
		t.Fatal("objSearch error")
	}
}
