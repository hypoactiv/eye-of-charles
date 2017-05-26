package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"sync"
	"time"

	"github.com/hypoactiv/imutil"
	"github.com/hypoactiv/objsearch"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// command line arguments
var (
	eoc           = kingpin.New("eye-of-charles", "A simple computer vision tool for finding objects amongst fields. Writes a list of (X,Y) hits with confidence scores to out.csv if they are within the given tolerance.")
	imageFilename = []*string{
		eoc.Arg("field", "field (game screen) image").Required().ExistingFile(),
		eoc.Arg("object", "object image to find in field").Required().ExistingFile(),
	}
	tolerance     = eoc.Flag("tolerance", "only output hits with score below this tolerance.").Short('T').Default("0").Float64()
	hitDist       = eoc.Flag("dist", "minimum pixel distance between hits. if negative, use object image size.").Default("-1").Int()
	verbose       = eoc.Flag("verbose", "verbose output").Short('v').Bool()
	pngOutput     = eoc.Flag("png", "enable PNG debug output").Bool()
	noCsvOutput   = eoc.Flag("no-csv", "disable CSV output").Bool()
	timeoutMillis = eoc.Flag("timeout", "timeout in milliseconds. 0 to disable timeout.").Short('t').Default("0").Uint()
	hitOffset     = Point(eoc.Flag("offset", "apply offset to all hits. default: hits are centered on object.").PlaceHolder("X,Y").Default("0,0"))
	searchRect    = Rectangle(eoc.Flag("rect", "search for hits only within this rectangle of the field image.").PlaceHolder("X_MIN,Y_MIN,X_MAX,Y_MAX"))
)

// imageFilename slice indices
const (
	IMG_FIELD = iota
	IMG_OBJECT
)

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

// output only if verbose output desired
func verboseOut(format string, a ...interface{}) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func main() {
	var img [2]*image.RGBA
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
			var ok bool
			if img[w], ok = tmpImg.(*image.RGBA); !ok {
				verboseOut("warning: converting image %s format %T to RGBA\n", *imageFilename[w], tmpImg)
				img[w] = imutil.ToRGBA(tmpImg)
			}
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
	var verboseWriter io.Writer
	if *verbose {
		verboseWriter = os.Stderr
	}
	hits := objsearch.Search(
		img[IMG_FIELD],
		img[IMG_OBJECT],
		*searchRect,
		*tolerance,
		*hitDist,
		verboseWriter,
		objsearch.COLORMODE_GRAY,
		objsearch.COMBINEMODE_MAX,
	)
	// offset hits
	for i := range hits {
		hits[i].P = hits[i].P.Add(*hitOffset)
	}
	// output hits
	for i := range hits {
		s := fmt.Sprintf("%d,%d,%f", hits[i].P.X, hits[i].P.Y, hits[i].S)
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
			draw.Draw(img[IMG_FIELD], image.Rect(h.P.X-5, h.P.Y-5, h.P.X+5, h.P.Y+5), &image.Uniform{color.Gray{0}}, image.ZP, draw.Src)
		}
		png.Encode(pngOut, img[IMG_FIELD])
	}
}
