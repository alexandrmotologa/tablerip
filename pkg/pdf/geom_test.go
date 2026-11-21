package pdf

import (
	"testing"
)

func TestBBoxOperations(t *testing.T) {
	b1 := NewBBox(10, 20, 50, 60)
	if b1.Width() != 40 {
		t.Fatalf("expected width 40, got %f", b1.Width())
	}
	if b1.Height() != 40 {
		t.Fatalf("expected height 40, got %f", b1.Height())
	}
	if b1.Area() != 1600 {
		t.Fatalf("expected area 1600, got %f", b1.Area())
	}

	b2 := NewBBox(30, 40, 70, 80)
	if !b1.Intersects(b2) {
		t.Fatalf("expected b1 to intersect b2")
	}

	inter := b1.Intersection(b2)
	expectedInter := NewBBox(30, 40, 50, 60)
	if inter != expectedInter {
		t.Fatalf("expected intersection %v, got %v", expectedInter, inter)
	}

	union := b1.Union(b2)
	expectedUnion := NewBBox(10, 20, 70, 80)
	if union != expectedUnion {
		t.Fatalf("expected union %v, got %v", expectedUnion, union)
	}

	b3 := NewBBox(100, 100, 120, 120)
	if b1.Intersects(b3) {
		t.Fatalf("expected b1 not to intersect b3")
	}
}

func TestRulingLines(t *testing.T) {
	rects := []BBox{
		NewBBox(10, 50, 200, 51),  // Horizontal thin line (thickness 1.0)
		NewBBox(200, 50, 400, 51), // Continuation of horizontal line
		NewBBox(50, 10, 51, 300),  // Vertical line (thickness 1.0)
		NewBBox(100, 100, 150, 150), // Fat box, not a ruling line
	}

	hLines, vLines := ExtractRulingLines(rects, 3.0)

	// After merge, horizontal lines should merge into one long line from X=10 to X=400
	if len(hLines) != 1 {
		t.Fatalf("expected 1 merged horizontal line, got %d", len(hLines))
	}
	if hLines[0].P0.X != 10 || hLines[0].P1.X != 400 {
		t.Fatalf("expected line from 10 to 400, got %f to %f", hLines[0].P0.X, hLines[0].P1.X)
	}

	if len(vLines) != 1 {
		t.Fatalf("expected 1 vertical line, got %d", len(vLines))
	}
	if vLines[0].P0.Y != 10 || vLines[0].P1.Y != 300 {
		t.Fatalf("expected line from Y 10 to 300, got %f to %f", vLines[0].P0.Y, vLines[0].P1.Y)
	}
}

func TestStandardCharWidth(t *testing.T) {
	wDigit := standardCharWidth('5', "Helvetica")
	if wDigit != 556.0 {
		t.Errorf("expected digit width 556.0, got %f", wDigit)
	}

	wSpace := standardCharWidth(' ', "Helvetica")
	if wSpace != 278.0 {
		t.Errorf("expected space width 278.0, got %f", wSpace)
	}

	wCourier := standardCharWidth('A', "Courier")
	if wCourier != 600.0 {
		t.Errorf("expected courier monospaced width 600.0, got %f", wCourier)
	}
}
