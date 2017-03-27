//-----------------------------------------------------------------------------
/*

Create curves using Bezier splines.

*/
//-----------------------------------------------------------------------------

package sdf

import (
	"fmt"
	"math"
)

//-----------------------------------------------------------------------------

const POLY_EPSILON = 1e-12

//-----------------------------------------------------------------------------

type BezierPolynomial struct {
	n             int     // polynomial order
	a, b, c, d, e float64 // polynomial coefficients
}

// Return the bezier polynomial function value.
func (p *BezierPolynomial) f0(t float64) float64 {
	switch p.n {
	case 1:
		// linear
		return p.a + t*p.b
	case 2:
		// quadratic
		return p.a + t*(p.b+t*p.c)
	case 3:
		// cubic
		return p.a + t*(p.b+t*(p.c+t*p.d))
	case 4:
		// quartic
		return p.a + t*(p.b+t*(p.c+t*(p.d+t*p.e)))
	default:
		panic(fmt.Sprintf("bad polynomial order %d", p.n))
	}
}

// Return the 1st derivative of the bezier polynomial.
func (p *BezierPolynomial) f1(t float64) float64 {
	switch p.n {
	case 1:
		// linear
		return p.b
	case 2:
		// quadratic
		return p.b + t*2*p.c
	case 3:
		// cubic
		return p.b + t*(2*p.c+t*3*p.d)
	case 4:
		// quartic
		return p.b + t*(2*p.c+t*(3*p.d+t*4*p.e))
	default:
		panic(fmt.Sprintf("bad polynomial order %d", p.n))
	}
}

// Return the 2nd derivative of the bezier polynomial.
func (p *BezierPolynomial) f2(t float64) float64 {
	switch p.n {
	case 1:
		// linear
		return 0
	case 2:
		// quadratic
		return 2 * p.c
	case 3:
		// cubic
		return 2 * (p.c + t*3*p.d)
	case 4:
		// quartic
		return 2 * (p.c + t*3*(p.d+t*2*p.e))
	default:
		panic(fmt.Sprintf("bad polynomial order %d", p.n))
	}
}

// Given the end/control points calculate the polynomial coefficients.
func (p *BezierPolynomial) Set(x []float64) {
	p.n = len(x) - 1
	switch p.n {
	case 1:
		// linear
		p.a = x[0]
		p.b = -x[0] + x[1]
	case 2:
		// quadratic
		p.a = x[0]
		p.b = -2*x[0] + 2*x[1]
		p.c = x[0] - 2*x[1] + x[2]
	case 3:
		// cubic
		p.a = x[0]
		p.b = -3*x[0] + 3*x[1]
		p.c = 3*x[0] - 6*x[1] + 3*x[2]
		p.d = -x[0] + 3*x[1] - 3*x[2] + x[3]
	case 4:
		// quartic
		p.a = x[0]
		p.b = -4*x[0] + 4*x[1]
		p.c = 6*x[0] - 12*x[1] + 6*x[2]
		p.d = -4*x[0] + 12*x[1] - 12*x[2] + 4*x[3]
		p.e = x[0] - 4*x[1] + 6*x[2] - 4*x[3] + x[4]
	default:
		panic(fmt.Sprintf("bad polynomial order %d", p.n))
	}
	// zero out any very small coefficients
	sum := Abs(p.a) + Abs(p.b) + Abs(p.c) + Abs(p.d) + Abs(p.e)
	p.a = ZeroSmall(p.a, sum, POLY_EPSILON)
	p.b = ZeroSmall(p.b, sum, POLY_EPSILON)
	p.c = ZeroSmall(p.c, sum, POLY_EPSILON)
	p.d = ZeroSmall(p.d, sum, POLY_EPSILON)
	p.e = ZeroSmall(p.e, sum, POLY_EPSILON)
}

//-----------------------------------------------------------------------------

type BezierSpline struct {
	px, py BezierPolynomial // x/y bezier polynomials
}

// Return the function value for a given t value.
func (s *BezierSpline) f0(t float64) V2 {
	return V2{s.px.f0(t), s.py.f0(t)}
}

// Return the curve slope (as an angle) for a given t value.
func (s *BezierSpline) slope(t float64) float64 {
	return math.Atan2(s.py.f1(t), s.px.f1(t))
}

// Return the rate of change of curve slope for a given t value.
func (s *BezierSpline) m1(t float64) (float64, error) {
	x1 := s.px.f1(t)
	y1 := s.py.f1(t)
	x2 := s.px.f2(t)
	y2 := s.py.f2(t)
	if x1 == 0 {
		return 0, fmt.Errorf("inf")
	}
	return (x1*y2 - y1*x2) / x1 * x1, nil
}

// Return the order of the bezier polynomial.
func (s *BezierSpline) order() int {
	return s.px.n
}

func NewBezierSpline(p []V2) *BezierSpline {
	//fmt.Printf("%v\n", p)
	s := BezierSpline{}
	// work out the polynomials
	x := make([]float64, len(p))
	y := make([]float64, len(p))
	for i, v := range p {
		x[i] = v.X
		y[i] = v.Y
	}
	s.px.Set(x)
	s.py.Set(y)
	return &s
}

//-----------------------------------------------------------------------------

type BezierVertexType int

const (
	ENDPOINT BezierVertexType = iota // endpoint
	MIDPOINT                         // midpoint
)

type BezierVertex struct {
	vtype      BezierVertexType // type of bezier vertex
	vertex     V2               // vertex coordinates
	handle_fwd V2               // polar coordinates of forward handle
	handle_rev V2               // polar coordinates of reverse handle
}

type Bezier struct {
	closed bool           // is the curve closed or open?
	vlist  []BezierVertex // list of bezier vertices
}

//-----------------------------------------------------------------------------

// Convert handles to control points.
func (b *Bezier) handles() {
	// new control vertex list
	var vlist []BezierVertex
	for _, v := range b.vlist {
		fwd := v.handle_fwd
		rev := v.handle_rev
		v.handle_fwd = V2{}
		v.handle_rev = V2{}
		// add a control midpoint for the reverse handle
		if rev.X != 0 {
			cp := BezierVertex{}
			cp.vtype = MIDPOINT
			cp.vertex = PolarToXY(rev.X, rev.Y).Add(v.vertex)
			vlist = append(vlist, cp)
		}
		// add the original curve end point.
		vlist = append(vlist, v)
		// add a control midpoint for the forward handle
		if fwd.X != 0 {
			cp := BezierVertex{}
			cp.vtype = MIDPOINT
			cp.vertex = PolarToXY(fwd.X, fwd.Y).Add(v.vertex)
			vlist = append(vlist, cp)
		}
	}
	// find the first endpoint control vertex
	i := 0
	for i = range vlist {
		if vlist[i].vtype == ENDPOINT {
			break
		}
	}
	// move any leading midpoints to the end of the list
	if i != 0 {
		vlist = append(vlist[i:], vlist[:i]...)
	}
	// replace the original control vertex list
	b.vlist = vlist
}

// Take care of curve closure.
func (b *Bezier) closure() {
	// do we need to close the curve?
	if !b.closed {
		return
	}
	first := b.vlist[0]
	last := b.vlist[len(b.vlist)-1]
	if first.vtype != ENDPOINT {
		panic("first control vertex should be an endpoint")
	}
	if last.vtype == ENDPOINT {
		if !last.vertex.Equals(first.vertex, 0) {
			// the first and last vertices aren't equal.
			// add the first vertex to close the curve
			b.vlist = append(b.vlist, first)
		}
	} else if last.vtype == MIDPOINT {
		// add the first vertex to close the curve
		b.vlist = append(b.vlist, first)
	} else {
		panic("bad vertex type")
	}
}

// Do some validation checks on the control verticesake care of curve closure.
func (b *Bezier) validate() {
	// basic checks
	n := len(b.vlist)
	if n < 2 {
		panic("bezier curve must have at least two points")
	}
	if b.vlist[0].vtype != ENDPOINT {
		panic("bezier curve must start with an endpoint")
	}
	if !b.closed && b.vlist[n-1].vtype != ENDPOINT {
		panic("non-closed bezier curve must end with an endpoint")
	}
}

// Post definition control point fixups.
func (b *Bezier) fixups() {
	b.handles()
	b.closure()
	b.validate()
}

//-----------------------------------------------------------------------------
// Public API for Bezier Curves.

// Returns an empty bezier curve.
func NewBezier() *Bezier {
	return &Bezier{}
}

// Close the bezier curve.
func (b *Bezier) Close() {
	b.closed = true
}

// Add a V2 vertex to a polygon.
func (b *Bezier) AddV2(x V2) *BezierVertex {
	v := BezierVertex{}
	v.vertex = x
	v.vtype = ENDPOINT
	b.vlist = append(b.vlist, v)
	return &b.vlist[len(b.vlist)-1]
}

// Add an x,y vertex to a polygon.
func (b *Bezier) Add(x, y float64) *BezierVertex {
	return b.AddV2(V2{x, y})
}

// Mid marks the vertex as a mid-curve control point.
func (v *BezierVertex) Mid() *BezierVertex {
	v.vtype = MIDPOINT
	return v
}

// Set the slope handle in the forward direction.
func (v *BezierVertex) HandleFwd(theta, r float64) *BezierVertex {
	if v.vtype == MIDPOINT {
		panic("can't place a handle on a curve midpoint")
	}
	v.handle_fwd = V2{Abs(r), theta}
	return v
}

// Set the slope handle in the reverse direction.
func (v *BezierVertex) HandleRev(theta, r float64) *BezierVertex {
	if v.vtype == MIDPOINT {
		panic("can't place a handle on a curve midpoint")
	}
	v.handle_rev = V2{Abs(r), theta}
	return v
}

// Handle marks the vertex with a slope control handle.
func (v *BezierVertex) Handle(theta, fwd, rev float64) *BezierVertex {
	v.HandleFwd(theta, fwd)
	v.HandleRev(theta+PI, rev)
	return v
}

// Return a polygon approximating the bezier curve.
func (b *Bezier) Polygon() *Polygon {
	b.fixups()

	// generate the splines from the vertices
	var splines []*BezierSpline
	var vertices []V2

	n := len(b.vlist)
	state := ENDPOINT
	i := 0
	for i < n {
		v := b.vlist[i]
		if state == ENDPOINT {
			if v.vtype == ENDPOINT {
				// start of spline
				vertices = []V2{v.vertex}
				// get the midpoints
				i += 1
				state = MIDPOINT
			} else {
				panic("bad vertex type")
			}
		} else if state == MIDPOINT {
			if v.vtype == ENDPOINT {
				// end of spline
				vertices = append(vertices, v.vertex)
				splines = append(splines, NewBezierSpline(vertices))
				// this endpoint is the start of the next spline, don't advance
				state = ENDPOINT
				// check for the last endpoint
				if i == n-1 {
					// end of the list
					break
				}
			} else if v.vtype == MIDPOINT {
				// add a spline midpoint
				vertices = append(vertices, v.vertex)
				i += 1
			} else {
				panic("bad vertex type")
			}
		} else {
			panic("bad state")
		}
	}

	// render the splines to a polygon
	p := NewPolygon()
	k := 1000
	dtmin := 1.0 / float64(k-1)
	epsilon := 0.1

	for _, s := range splines {

		if s.order() == 1 {
			// linear
			p.AddV2(s.f0(0))
			p.AddV2(s.f0(1))
		} else {
			t := 0.0
			for t < 1.0 {
				p.AddV2(s.f0(t))
				dtheta := Abs(s.slope(t+dtmin) - s.slope(t))
				if dtheta < epsilon {
					t += dtmin * (epsilon / dtheta)
				} else {
					t += dtmin
				}
			}
			p.AddV2(s.f0(1))

		}
	}
	return p
}

//-----------------------------------------------------------------------------
