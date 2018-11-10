package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	_ "regexp"
	"runtime"
	_ "strings"
	"image"
	"image/draw"
	"image/color"
	"image/png"

	"ansimage"

	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/crypto/ssh/terminal"

	"golang.org/x/image/font"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)


var (
	flagDither  uint
	flagMatte   string
	flagRows    uint
	flagCols    uint
	flagFont	string
)

var (
	imgWidth,  imgHeight int
	termWidth, termHeight int
	srcImageFile string
	infoImageFile string
	concatImageFile string
	workDir string
	dataDir string
)

var dpi = flag.Float64("dpi", 256, "screen resolution")


func init() {
	configureFlags()
}

func main() {
	validateFlags()
	srcImageFile = flag.CommandLine.Arg(0)
	termWidth, termHeight = GetTermSize()
	imgWidth,  imgHeight = GetImageSize(srcImageFile)
	workDir = getBinaryDir()
	dataDir = workDir+"/../data/"
	infoImageFile = dataDir+"info.image.png"
	concatImageFile = dataDir+"fetch.image.png"
	
	createInfoImage()
	concatImage(srcImageFile, infoImageFile)
	RenderTerm(concatImageFile)
}

func throwError(code int, v ...interface{}) {
        log.New(os.Stderr, "Error: ", log.LstdFlags).Println(v...)
        os.Exit(code)
}

func configureFlags() {
	flag.CommandLine.Usage = func() {

		_, file := filepath.Split(os.Args[0])
		fmt.Print("USAGE:\n\n")
		fmt.Printf("  %s [options] image/url\n\n", file)

		fmt.Print("  Supported image formats: JPEG, PNG, GIF, BMP, TIFF, WebP.\n\n")
		//fmt.Print("  Supported URL protocols: HTTP, HTTPS.\n\n")

		fmt.Print("OPTIONS:\n\n")
		flag.CommandLine.SetOutput(os.Stdout)
		flag.CommandLine.PrintDefaults()
		flag.CommandLine.SetOutput(ioutil.Discard) // hide flag errors
		fmt.Print("  -help\n\tprints this message :D LOL\n")
		fmt.Println()
	}

	flag.CommandLine.SetOutput(ioutil.Discard) // hide flag errors
	flag.CommandLine.Init(os.Args[0], flag.ExitOnError)

	flag.CommandLine.UintVar(&flagDither, "d", 0, "dithering `mode`:\n   \t0 - no dithering (default)\n   \t1 - with blocks\n   \t2 - with chars")
	flag.CommandLine.StringVar(&flagMatte, "m", "", "matte `color` for transparency or background\n\t(optional, hex format, default: 000000)")
	flag.CommandLine.UintVar(&flagRows, "tr", 24, "terminal `rows` (optional, >=2)")
	flag.CommandLine.UintVar(&flagCols, "tc", 80, "terminal `columns` (optional, >=2)")
	flag.CommandLine.StringVar(&flagFont, "f", "open.sans.700.ttf", "the `font family` file path (optional, default: open.sans.700.ttf)")

	flag.CommandLine.Parse(os.Args[1:])
}

func validateFlags() {


	if flagDither != 0 && flagDither != 1 && flagDither != 2 {
		flag.CommandLine.Usage()
		os.Exit(2)
	}


	if (flagRows > 0 && flagRows < 2) || (flagCols > 0 && flagCols < 2) {
		flag.CommandLine.Usage()
		os.Exit(2)
	}

	// this is image filename
	if flag.CommandLine.Arg(0) == "" {
		flag.CommandLine.Usage()
		os.Exit(2)
	}
}

func getBinaryDir() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
    if err != nil {
            log.Fatal(err)
    }
    return dir 
}

func isTerminal() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func getTerminalSize() (width, height int, err error) {
	if isTerminal() {
		return terminal.GetSize(int(os.Stdout.Fd()))
	}
	// fallback when piping to a file!
	return 80, 24, nil // VT100 terminal size
}

func concatImage(leftFile, rightFile string) {

	// Load two images files
	left_fp, err := os.Open(leftFile)
	right_fp, err := os.Open(rightFile)
	if err != nil {
	    throwError(255, err)
	}
	left_img, _, err := image.Decode(left_fp)
	right_img, _, err := image.Decode(right_fp)
	if err != nil {
	    throwError(255, err)
	}


	////starting position of the second image (bottom left)
	right_sp := image.Point{left_img.Bounds().Dx(), 0}

	//new rectangle for the second image
	right_rect := image.Rectangle{right_sp, right_sp.Add(right_img.Bounds().Size())}

	//rectangle for the big image
	rect := image.Rectangle{image.Point{0, 0}, right_rect.Max}

	//create the new Image file
	rgba := image.NewRGBA(rect)

	draw.Draw(rgba, left_img.Bounds(), left_img, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, right_rect, right_img, image.Point{0, 0}, draw.Src)

	// Encode as PNG.
	f, _ := os.Create(concatImageFile)
	png.Encode(f, rgba)
	f.Close()
}

func addLabel(img *image.RGBA, x, y int, size float64, c color.RGBA, label string) {
/*    col := color.RGBA{200, 100, 0, 255}
    point := fixed.Point26_6{fixed.Int26_6(x * 64), fixed.Int26_6(y * 64)}

    d := &font.Drawer{
        Dst:  img,
        Src:  image.NewUniform(col),
        Face: basicfont.Face7x13,
        Dot:  point,
    }
    d.DrawString(label)*/

    ctx := freetype.NewContext()

    ctx.SetDPI(*dpi)
    ctx.SetClip(img.Bounds())
    ctx.SetDst(img)
    ctx.SetHinting(font.HintingFull)

    // set font color, size and family
    ctx.SetSrc(image.NewUniform(c))
    //c.SetSrc(image.White)
    ctx.SetFontSize(size)
    fontFam, err := getFontFamily()
    if err != nil {
        fmt.Println("get font family error")
    }
    ctx.SetFont(fontFam)

    pt := freetype.Pt(x, y)

    _, err = ctx.DrawString(label, pt)
    if err != nil {
        fmt.Printf("draw error: %v \n", err)
    }
}

func getFontFamily() (*truetype.Font, error) {
    
    fontBytes, err := ioutil.ReadFile(dataDir + flagFont)
    if err != nil {
        fmt.Println("read file error:", err)
        return &truetype.Font{}, err
    }

    f, err := freetype.ParseFont(fontBytes)
    if err != nil {
        throwError(255, err)
        return &truetype.Font{}, err
    }

    return f, err
}

func createInfoImage() {
	width, height := imgWidth, imgHeight
	 

	upLeft := image.Point{0, 0}
	lowRight := image.Point{width, height}

	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})


	host, _ := os.Hostname()
	addLabel(img, 10, 50, 11, color.RGBA{R: 255, G: 255, B: 255, A: 255}, host);
	
	user, _ := user.Current()
	addLabel(img, 10, 120, 14, color.RGBA{R: 255, G: 255, B: 255, A: 255}, user.Username);

	addLabel(img, 10, 180, 9, color.RGBA{R: 255, G: 255, B: 255, A: 255}, runtime.GOOS +"/"+runtime.GOARCH);

	//addLabel(img, 10, 200, 11, color.RGBA{R: 255, G: 255, B: 255, A: 255}, "");

	

	// Encode as PNG.
	f, _ := os.Create(infoImageFile)
	png.Encode(f, img)
	f.Close()
}

func GetTermSize() (w, h int) {
		// get terminal size
	tx, ty, err := getTerminalSize()
	if err != nil {
		throwError(1, err)
	}

	// use custom terminal size (if applies)
	if ty--; flagRows != 0 { // no custom rows? subtract 1 for prompt spacing
		ty = int(flagRows) + 1 // weird, but in this case is necessary to add 1 :O
	}
	if flagCols != 0 {
		tx = int(flagCols)
	}
	return tx, ty
}


func GetImageSize(imageFile string) (int, int) {
    file, err := os.Open(imageFile)
    defer file.Close()
    if err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err)
    }

    image, _, err := image.DecodeConfig(file)
    if err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", imageFile, err)
    }
    return image.Width, image.Height
}

func RenderTerm(file string) {
	var (
		pix *ansimage.ANSImage
		err error
	)

	tx, ty := termWidth*2, termHeight

	// get scale mode from flag
	sm := ansimage.ScaleMode(0)

	// get dithering mode from flag
	dm := ansimage.DitheringMode(flagDither)

	// set image scale factor for ANSIPixel grid
	sfy, sfx := ansimage.BlockSizeY, ansimage.BlockSizeX // 8x4 --> with dithering
	if ansimage.DitheringMode(flagDither) == ansimage.NoDithering {
		sfy, sfx = 2, 1 // 2x1 --> without dithering
	}

	// get matte color
	if flagMatte == "" {
		flagMatte = "000000" // black background
	}
	mc, err := colorful.Hex("#" + flagMatte) // RGB color from Hex format
	if err != nil {
		throwError(2, fmt.Sprintf("matte color : %s is not a hex-color", flagMatte))
	}

	// create new ANSImage from file
	pix, err = ansimage.NewScaledFromFile(file, sfy*ty, sfx*tx, mc, sm, dm)

	/*if matched, _ := regexp.MatchString(`^https?://`, file); matched {
		pix, err = ansimage.NewScaledFromURL(file, sfy*ty, sfx*tx, mc, sm, dm)
	} else {
		pix, err = ansimage.NewScaledFromFile(file, sfy*ty, sfx*tx, mc, sm, dm)
	}*/
	if err != nil {
		throwError(1, err)
	}

	// draw ANSImage to terminal
	if isTerminal() {
		ansimage.ClearTerminal()
	}
	pix.SetMaxProcs(runtime.NumCPU()) // maximum number of parallel goroutines!
	pix.Draw()
	if isTerminal() {
		fmt.Println()
	}
}


