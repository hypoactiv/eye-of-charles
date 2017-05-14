// Compute the convlution (cross-correlation) of two image files and output
// the result

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sync"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// command line arguments
var (
	imageFilename = []*string{
		kingpin.Arg("field", "field (game screen) image").Required().ExistingFile(),
		kingpin.Arg("object", "object to find").Required().ExistingFile(),
	}
	threshholdLow  = kingpin.Flag("low", "output hits below this thresshold.").Default("0").Float64()
	threshholdHigh = kingpin.Flag("high", "output hits above this threshhold.").Default("1").Float64()
	hitDist        = kingpin.Flag("dist", "minimum pixel distance between hits. if negative, use object image size.").Default("-1").Int()
	verbose        = kingpin.Flag("verbose", "verbose output").Short('v').Bool()
	pngOutput      = kingpin.Flag("png", "enable png output").Default("false").Bool()
)

// imageFilename slice indices
const (
	IMG_FIELD = iota
	IMG_OBJECT
)

type Point struct {
	X, Y int
}

// Distance (L-inf norm) between Points p and q
func (p Point) Dist(q Point) int {
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

// matches to be output
var hits []Point

func verboseOut(format string, a ...interface{}) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func toGrayscale(i image.Image) image.Image {
	r := i.Bounds()
	gray := image.NewGray(r)
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			_, g, _, a := i.At(x, y).RGBA()
			c := color.RGBA{0, uint8(g >> 8), 0, uint8(a >> 8)}
			gray.Set(x, y, color.GrayModel.Convert(c))
		}
	}
	return gray
}

// add hit p to hits list
func addHit(p Point, pxl float64) {
	if hitDist != nil {
		// is this hit p too close to some other hit?
		for i := range hits {
			if hits[i].Dist(p) < *hitDist {
				// too close to previous hits[i], drop hit p
				// TODO if p is a better hit, replace existing hit?
				return
			}
		}
	}
	hits = append(hits, p)
	// TODO output only best hits at end of run
	fmt.Printf("%d,%d,%f\n", p.X, p.Y, pxl)
}

func grayConvolve(f, g *image.Gray) image.Image {
	out := image.NewGray(f.Rect)
	float := func(img *image.Gray, x, y int) float64 {
		return float64(img.GrayAt(x, y).Y) / 255.0
	}
	/*
		mean := func(img *image.Gray) float64 {
			sum := 0.0
			size := img.Rect.Size()
			area := float64(size.X * size.Y)
			for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
				for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
					sum += float(img, x, y)
				}
			}
			return sum / area
		}
		fMean := mean(f)
		gMean := mean(g)
	*/
	wg := sync.WaitGroup{}
	outFloat := make([]float64, len(out.Pix))
	convolve1 := func(u, v int) {
		result := 0.0
		i := out.PixOffset(u, v)
		out.Pix[i] = 0
		for x := g.Rect.Min.X; x < g.Rect.Max.X; x++ {
			for y := g.Rect.Min.Y; y < g.Rect.Max.Y; y++ {
				result += math.Abs(float(f, u+x, v+y) - float(g, x, y)) //(float(f, u+x, v+y) - fMean) * (float(g, x, y) - gMean)
			}
		}
		outFloat[i] = result
		//out.Pix[i] = uint8(2*result + 64)
		//fmt.Println(result)
		wg.Done()
	}
	verboseOut("\n")
	for u := out.Rect.Min.X; u < out.Rect.Max.X; u++ {
		wg.Add(out.Rect.Size().Y)
		for v := out.Rect.Min.Y; v < out.Rect.Max.Y; v++ {
			go convolve1(u, v)
		}
		wg.Wait()
		verboseOut("\r%.2f%% complete", float64(u)/float64(out.Rect.Size().X)*100)
	}
	verboseOut("\n")
	// compute min and max of image pixels
	min := outFloat[0]
	max := outFloat[0]
	for u := out.Rect.Min.X; u < out.Rect.Max.X; u++ {
		for v := out.Rect.Min.Y; v < out.Rect.Max.Y; v++ {
			pxl := outFloat[out.PixOffset(u, v)]
			if pxl < min {
				min = pxl
			}
			if pxl > max {
				max = pxl
			}
		}
	}
	// produce output image
	pxlRange := max - min
	for u := out.Rect.Min.X; u < out.Rect.Max.X; u++ {
		for v := out.Rect.Min.Y; v < out.Rect.Max.Y; v++ {
			i := out.PixOffset(u, v)
			pxl := ((outFloat[i] - min) / pxlRange)
			if pxl < *threshholdLow {
				addHit(Point{u, v}, pxl)
			}
			if pxl > *threshholdHigh {
				addHit(Point{u, v}, pxl)
			}
			out.Pix[i] = uint8(pxl * 255)
		}
	}
	return out
}

func main() {
	var img [2]image.Image
	var wg sync.WaitGroup
	kingpin.Version("test")
	kingpin.Parse()
	if *threshholdLow == 0 && *threshholdHigh == 1 {
		fmt.Fprint(os.Stderr, "Warning: command line sets --low=0 and --high=1, so no hits will be output.\n")
		if *pngOutput == false {
			fmt.Fprint(os.Stderr, "Error: PNG output is also disabled. Nothing to do!\n")
			fmt.Fprint(os.Stderr, "Threshhold values --low and/or --high must be set, or PNG output enabled with --png\n")
			return
		}
	}
	verboseOut("starting\n")
	defer verboseOut("exiting\n")
	wg.Add(len(imageFilename))
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
			img[w], err = png.Decode(f) // TODO call a decodeImage function that supports other file types
			if err != nil {
				fmt.Fprintf(os.Stderr, "error decoding %s\n", *imageFilename[w])
				os.Exit(1)
				return
			}
			img[w] = toGrayscale(img[w]) // TODO allow preprocessing other than grayscale
			verboseOut("loaded %s\n", *imageFilename[w])
			f.Close()
		}(i)
	}
	wg.Wait()
	verboseOut("loading images took %v\n", time.Since(loadImageStartTime))
	// image1 and image2 are loaded in img
	//
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
	verboseOut("minimum hit distance %d", *hitDist)
	switch u := img[0].(type) {
	case *image.Gray:
		out := grayConvolve(u, img[1].(*image.Gray))
		outFile, err := os.OpenFile("out.png", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to open output file: %s\n", err.Error())
		}
		png.Encode(outFile, out)
		outFile.Close()
	default:
		fmt.Fprintf(os.Stderr, "unexpected image format %T\n", u)
		os.Exit(1)
		return
	}
}
