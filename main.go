package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// command line arguments
var (
	eoc           = kingpin.New("eye-of-charles", "A simple computer vision tool for finding objests amongst fields. Writes a list of (X,Y) hits with confidence scores to out.csv if they are within the given tolerance.")
	imageFilename = []*string{
		eoc.Arg("field", "field (game screen) image").Required().ExistingFile(),
		eoc.Arg("object", "object image to find in field").Required().ExistingFile(),
	}
	tolerance     = eoc.Flag("tolerance", "only output hits with score below this tolerance.").Short('T').Default("0").Float64()
	hitDist       = eoc.Flag("dist", "minimum pixel distance between hits. if negative, use object image size.").Default("-1").Int()
	verbose       = eoc.Flag("verbose", "verbose output").Short('v').Bool()
	pngOutput     = eoc.Flag("png", "enable PNG debug output").Bool()
	noCsvOutput   = eoc.Flag("no-csv", "disable CSV output").Bool()
	hitOffset     = Point(eoc.Flag("offset", "apply offset to all hits. default: hits are centered on object.").PlaceHolder("X,Y").Default("0,0"))
	timeoutMillis = eoc.Flag("timeout", "timeout in milliseconds. 0 to disable timeout.").Short('t').Default("0").Uint()
	searchRect    = Rectangle(eoc.Flag("rect", "search for hits only within this rectangle of the field image.").PlaceHolder("X_MIN,Y_MIN,X_MAX,Y_MAX"))
)

// imageFilename slice indices
const (
	IMG_FIELD = iota
	IMG_OBJECT
)

type Hit struct {
	X, Y int
	P    float64
}

// Command line point parser
type PointValue image.Point

func (p *PointValue) Set(value string) error {
	_, err := fmt.Sscanf(value, "%d,%d", &p.X, &p.Y)
	return err
}

func (p *PointValue) String() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}

func Point(s kingpin.Settings) (target *image.Point) {
	target = &image.Point{}
	s.SetValue((*PointValue)(target))
	return
}

// Command line rectangle parser
type RectangleValue image.Rectangle

func (r *RectangleValue) Set(value string) error {
	_, err := fmt.Sscanf(value, "%d,%d,%d,%d", &r.Min.X, &r.Min.Y, &r.Max.X, &r.Max.Y)
	return err
}

func (r *RectangleValue) String() string {
	return fmt.Sprintf("%d,%d,%d,%d", r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
}

func Rectangle(s kingpin.Settings) (target *image.Rectangle) {
	target = &image.Rectangle{}
	s.SetValue((*RectangleValue)(target))
	return
}

// Distance (L-inf norm) between Hits p and q
func (p Hit) Dist(q Hit) int {
	dx := p.X - q.X
	dy := p.Y - q.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// output only if verbose mode is enabled
func verboseOut(format string, a ...interface{}) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func toGrayscale(i image.Image) *image.Gray {
	r := i.Bounds()
	gray := image.NewGray(r)
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			//_, g, _, a := i.At(x, y).RGBA()
			//c := color.RGBA{0, uint8(g >> 8), 0, uint8(a >> 8)}
			//gray.Set(x, y, color.GrayModel.Convert(c))
			gray.Set(x, y, color.GrayModel.Convert(i.At(x, y)))
		}
	}
	return gray
}

// add hit p to hits list
/*
func addHit(p Hit) {
	if hitDist != nil {
		// is this hit p too close to some other hit?
		for i := range hits {
			if hits[i].Dist(p) < *hitDist {
				// if p has a lower score, replace hits[i] with p. otherwise
				// discard hit p
				if p.P < hits[i].P {
					hits[i] = p
				}
				return
			}
		}
	}
	hits = append(hits, p)
}
*/

// return the slice index  corresponding to (x,y) in the search rectangle
func offset(searchRect image.Rectangle, x, y int) int {
	return (x - searchRect.Min.X) + searchRect.Dx()*(y-searchRect.Min.Y)
}

// return the (x,y) coordinates corresponding to slice index i
func coords(searchRect image.Rectangle, i int) (x, y int) {
	x = searchRect.Min.X + (i % searchRect.Dx())
	y = searchRect.Min.Y + (i / searchRect.Dx())
	return
}

func objSearch(field, object *image.Gray, searchRect image.Rectangle) (out []float64, min, max float64) {
	// convert the pixel at (x,y) in img to a float64
	float := func(img *image.Gray, x, y int) float64 {
		return float64(img.GrayAt(x, y).Y) / 255.0
	}
	wg := sync.WaitGroup{}
	out = make([]float64, searchRect.Dx()*searchRect.Dy())
	// compute the L1-norm distance between 'object' and and 'object'-sized
	// rectangle of 'field' with top-left corner at (u,v) in 'field'
	//
	// store result in out[offset(u,v)]
	objSearch1 := func(u, v int) {
		result := 0.0
		i := offset(searchRect, u, v)
		out[i] = 0
		// Compute L1-norm
		for x := object.Rect.Min.X; x < object.Rect.Max.X; x++ {
			for y := object.Rect.Min.Y; y < object.Rect.Max.Y; y++ {
				result += math.Abs(float(field, u+x, v+y) - float(object, x, y))
			}
		}
		out[i] = result
		wg.Done()
	}
	verboseOut("\n")
	// choose column of searchRect
	for u := searchRect.Min.X; u < searchRect.Max.X; u++ {
		// launch one goroutine per row of searchRect
		wg.Add(searchRect.Size().Y)
		for v := searchRect.Min.Y; v < searchRect.Max.Y; v++ {
			go objSearch1(u, v)
		}
		// wait for this column to finish
		wg.Wait()
		verboseOut("\r%.2f%% complete", float64(u-searchRect.Min.X)/float64(searchRect.Size().X)*100)
		// start next column
	}
	verboseOut("\n")
	// compute min and max of out
	min = out[0]
	max = out[0]
	for i := range out {
		if out[i] < min {
			min = out[i]
		}
		if out[i] > max {
			max = out[i]
		}
	}
	// done
	return
}

// verify command line options are valid
func verifyCmdLine() bool {
	if *tolerance == 0 {
		fmt.Fprint(os.Stderr, "Warning: --tolerance=0, so no hits will be output.\n")
		if *pngOutput == false {
			fmt.Fprint(os.Stderr, "Error: PNG output is also disabled, so there is nothing to do.\n")
			fmt.Fprint(os.Stderr, "Set --tolerance greater than 0, or enable PNG debug output with --png\n")
			return false
		}
	}
	if *searchRect != (image.Rectangle{}) && (searchRect.Dx() <= 0 || searchRect.Dy() <= 0) {
		fmt.Fprint(os.Stderr, "Error: search rectangle is zero or negative\n")
		return false
	}
	return true
}

// convert the float64 slice to a grayscale image
// value 'black' maps to black, value 'white' maps to white
func toImage(f []float64, black, white float64) *image.Gray {
	panic("not impl")
}

// Find L1 distances in d that are below t, and return them as a slice of Hits
// Only hits at least minDist apart are found
//
// d is a slice of distances from objSearch
// max is mapped to the Hit score of 1, and min is mapped to a Hit score of 0
// t is the L1 distance threshhold to produce a hit.
func findHits(d []float64, min, max, t float64, minDist int, searchRect image.Rectangle) (hits []Hit) {
	if max <= min {
		panic("max <= min")
	}
	hitChan := make(chan Hit)
	hits = make([]Hit, 0)
	dRange := max - min
	go func() {
		for i := range d {
			// normalize L1 distances into interval [0,1]
			p := (d[i] - min) / dRange
			if p < t {
				x, y := coords(searchRect, i)
				hitChan <- Hit{x, y, p}
			}
		}
		close(hitChan)
	}()
nextHit:
	for h := range hitChan {
		for j := range hits {
			if hits[j].Dist(h) < minDist {
				// h is too close to hits[j]
				// replace hits[j] if h's score is better, otherwise drop h
				if h.P < hits[j].P {
					hits[j] = h
				}
				continue nextHit
			}
		}
		// h is a new hit
		hits = append(hits, h)
	}
	// sort hits
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].P < hits[j].P
	})
	return
}

func main() {
	var img [2]*image.Gray
	var wg sync.WaitGroup
	kingpin.Version("test")
	kingpin.MustParse(eoc.Parse(os.Args[1:]))
	if !verifyCmdLine() {
		fmt.Fprint(os.Stderr, "exiting due to argument error\n")
		os.Exit(1)
		panic("unreachable")
	}
	verboseOut("starting\n")
	if *timeoutMillis > 0 {
		// start timeout timer
		go func() {
			time.Sleep(time.Duration(*timeoutMillis) * time.Millisecond)
			fmt.Fprint(os.Stderr, "exiting due to timeout\n")
			os.Exit(3)
		}()
	}
	var csv *os.File
	if *noCsvOutput == false {
		// create and open output csv file
		var err error
		csv, err = os.OpenFile("out.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open out.csv: %v\n", err)
			os.Exit(1)
			return
		}
		defer csv.Close()
	}
	defer verboseOut("exiting\n")
	wg.Add(len(imageFilename))
	// load field and object images
	loadImageStartTime := time.Now()
	for i := range imageFilename {
		go func(w int) {
			defer wg.Done()
			f, err := os.Open(*imageFilename[w])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error opening %s\n", *imageFilename[w])
				os.Exit(1)
				return
			}
			tmpImg, err := png.Decode(f) // TODO call a decodeImage function that supports other file types
			if err != nil {
				fmt.Fprintf(os.Stderr, "error decoding %s\n", *imageFilename[w])
				os.Exit(1)
				return
			}
			img[w] = toGrayscale(tmpImg) // TODO allow preprocessing other than grayscale
			verboseOut("loaded %s\n", *imageFilename[w])
			f.Close()
		}(i)
	}
	wg.Wait()
	verboseOut("loading images took %v\n", time.Since(loadImageStartTime))
	// image1 and image2 are now loaded in img
	//
	if *searchRect == (image.Rectangle{}) {
		// search rectangle is zero valued. set it to the entire field.
		*searchRect = img[IMG_FIELD].Bounds()
	}
	// hitOffset is from center of object in documentation
	hitOffset.X += img[IMG_OBJECT].Bounds().Dx() / 2
	hitOffset.Y += img[IMG_OBJECT].Bounds().Dy() / 2
	// subtract search rectangle by hitOffset, so resultant hits are in the
	// desired rectangle after adding hitOffset in the output step.
	*searchRect = searchRect.Sub(*hitOffset)
	verboseOut("search rectangle: %d\n", *searchRect)
	if *hitDist < 0 {
		// no minimum hit distance is set, use the larger of IMG_OBJECT's width
		// and height
		p := img[IMG_OBJECT].Bounds().Size()
		s := p.X
		if p.Y > s {
			s = p.Y
		}
		*hitDist = s
	}
	verboseOut("minimum hit distance %d\n", *hitDist)
	// perform object search
	out, _, max := objSearch(img[IMG_FIELD], img[IMG_OBJECT], *searchRect)
	// find hits
	hits := findHits(out, 0, max, *tolerance, *hitDist, *searchRect)
	if len(hits) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: no hits found\n")
		os.Exit(2)
		return
	}
	// offset hits
	for i := range hits {
		hits[i].X += hitOffset.X
		hits[i].Y += hitOffset.Y
	}
	// output hits
	for i := range hits {
		s := fmt.Sprintf("%d,%d,%f", hits[i].X, hits[i].Y, hits[i].P)
		if *noCsvOutput == false {
			csv.WriteString(fmt.Sprintln(s))
		}
		verboseOut("hit: %s\n", s)
	}
	if *pngOutput {
		// output debug png
		pngOut, err := os.Create("out.png")
		if err != nil {
			panic("unable to create output png")
		}
		defer pngOut.Close()
		// draw hits on field
		for _, h := range hits {
			draw.Draw(img[IMG_FIELD], image.Rect(h.X-5, h.Y-5, h.X+5, h.Y+5), &image.Uniform{color.Gray{0}}, image.ZP, draw.Src)
		}
		png.Encode(pngOut, img[IMG_FIELD])
	}
}
