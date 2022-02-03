package onmap_test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/dchest/onmap"
)

func TestPins(t *testing.T) {
	coords := []onmap.Coord{
		{42.1, 19.1},             // Bar
		{55.755833, 37.617222},   // Moscow
		{41.9097306, 12.2558141}, // Rome
		{-31.952222, 115.858889}, // Perth
		{42.441286, 19.262892},   // Podgorica
		{38.615925, -27.226598},  // Achores
		{45.4628329, 9.1076924},  // Milano
		{43.7800607, 11.170928},  // Florence
		{37.7775, -122.416389},   // San Francisco
	}

	m1 := onmap.Pins(coords, nil)
	if err := writePng("test-1.png", m1); err != nil {
		t.Fatal(err)
	}

	m2 := onmap.Pins(coords, onmap.StandardCrop)
	if err := writePng("test-2.png", m2); err != nil {
		t.Fatal(err)
	}

	m3 := onmap.Pins(coords[:3], onmap.StandardCrop)
	if err := writePng("test-3.png", m3); err != nil {
		t.Fatal(err)
	}

	m4 := onmap.Pins(coords[len(coords)-1:], onmap.StandardCrop)
	if err := writePng("test-4.png", m4); err != nil {
		t.Fatal(err)
	}

	fmt.Println("Test images are written, check them :)")
}

func writePng(filename string, m image.Image) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, m); err != nil {
		return err
	}
	return nil
}
