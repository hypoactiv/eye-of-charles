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
	"sort"
	"sync"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// command line arguments
var (
	eoc           = kingpin.New("eye-of-charles", "A simple computer vision tool for finding objests amongst fields. Outputs a list of hits (X, Y, and pixel values) that are above or below the provided threshholds.")
	imageFilename = []*string{
		eoc.Arg("field", "field (game screen) image").Required().ExistingFile(),
		eoc.Arg("object", "object to find").Required().ExistingFile(),
	}
	threshholdLow  = eoc.Flag("low", "output hits below this thresshold.").Default("0").Float64()
	threshholdHigh = eoc.Flag("high", "output hits above this threshhold.").Default("1").Float64()
	hitDist        = eoc.Flag("dist", "minimum pixel distance between hits. if negative, use object image size.").Default("-1").Int()
	verbose        = eoc.Flag("verbose", "verbose output").Short('v').Bool()
	pngOutput      = eoc.Flag("png", "enable PNG output").Default("false").Bool()
	csvOutput      = eoc.Flag("csv", "enable CSV output").Default("true").Bool()
	center         = eoc.Flag("center", "output center of object").Short('c').Default("true").Bool()
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

// matches to be output
var hits []Hit

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
			//_, g, _, a := i.At(x, y).RGBA()
			//c := color.RGBA{0, uint8(g >> 8), 0, uint8(a >> 8)}
			//gray.Set(x, y, color.GrayModel.Convert(c))
			gray.Set(x, y, color.GrayModel.Convert(i.At(x, y)))
		}
	}
	return gray
}

// add hit p to hits list
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

func grayConvolve(field, object *image.Gray) image.Image {
	out := image.NewGray(field.Rect)
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
		for x := object.Rect.Min.X; x < object.Rect.Max.X; x++ {
			for y := object.Rect.Min.Y; y < object.Rect.Max.Y; y++ {
				result += math.Abs(float(field, u+x, v+y) - float(object, x, y)) //(float(f, u+x, v+y) - fMean) * (float(g, x, y) - gMean)
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
	min = 0
	// produce output image
	pxlRange := max - min
	for u := out.Rect.Min.X; u < out.Rect.Max.X; u++ {
		for v := out.Rect.Min.Y; v < out.Rect.Max.Y; v++ {
			i := out.PixOffset(u, v)
			pxl := ((outFloat[i] - min) / pxlRange)
			p := Hit{u, v, pxl}
			if *center == true {
				p.X += object.Bounds().Size().X / 2
				p.Y += object.Bounds().Size().Y / 2
			}
			if pxl < *threshholdLow {
				addHit(p)
			}
			if pxl > *threshholdHigh {
				addHit(p)
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
	kingpin.MustParse(eoc.Parse(os.Args[1:]))
	if *threshholdLow == 0 && *threshholdHigh == 1 {
		fmt.Fprint(os.Stderr, "Warning: command line sets --low=0 and --high=1, so no hits will be output.\n")
		if *pngOutput == false {
			fmt.Fprint(os.Stderr, "Error: PNG output is also disabled. Nothing to do!\n")
			fmt.Fprint(os.Stderr, "Threshhold values --low and/or --high must be set, or PNG output enabled with --png\n")
			os.Exit(1)
			return
		}
	}
	verboseOut("starting\n")
	var csv *os.File
	if *csvOutput {
		var err error
		csv, err = os.OpenFile("out.csv", os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open out.csv: %v\n", err)
			os.Exit(1)
			return
		}
		defer csv.Close()
	}
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
		if *pngOutput {
			outFile, err := os.OpenFile("out.png", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unable to open output file: %s\n", err.Error())
			}
			png.Encode(outFile, out)
			outFile.Close()
		}
	default:
		fmt.Fprintf(os.Stderr, "unexpected image format %T\n", u)
		os.Exit(1)
		return
	}
	if len(hits) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: no hits found\n")
		os.Exit(2)
		return
	}
	// sort hits
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].P < hits[j].P
	})
	for i := range hits {
		s := fmt.Sprintf("%d,%d,%f", hits[i].X, hits[i].Y, hits[i].P)
		if *csvOutput {
			csv.WriteString(fmt.Sprintln(s))
		}
		fmt.Println("hit:", s)
	}
}
