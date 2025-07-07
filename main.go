package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

type Params struct {
	input  string
	output string
	blur   float64
	height int
	width  int
	radius int
	margin int
}

func bbf(params Params) int {
	fmt.Printf("Executing Job: %+v\n", params)

	if params.output == "" {
		params.output = "bbf_" + params.input
	}

	outputDir := filepath.Dir(params.output)
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Println("Failed to make output dir:", outputDir, err)
		return 1
	}

	src, err := imaging.Open(params.input)
	if err != nil {
		fmt.Println("Failed to open image:", params.input, err)
		return 1
	}

	bg := imaging.Fill(src, params.width, params.height, imaging.Center, imaging.Lanczos)
	bg = imaging.Blur(bg, params.blur)

	maxWidth := params.width - 2*params.margin
	maxHeight := params.height - 2*params.margin
	overlay := imaging.Fit(src, maxWidth, maxHeight, imaging.Lanczos)

	overlay = applyRoundedCorners(overlay, params.radius)

	x := (params.width - overlay.Bounds().Dx()) / 2
	y := (params.height - overlay.Bounds().Dy()) / 2
	result := imaging.Overlay(bg, overlay, image.Pt(x, y), 1.0)

	err = imaging.Save(result, params.output)
	if err != nil {
		fmt.Println("Failed to save output:", params.output, err)
		return 1
	}

	return 0
}

func applyRoundedCorners(img image.Image, radius int) *image.NRGBA {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	dst := imaging.New(w, h, color.Transparent)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			alpha := roundedRectAlpha(x, y, w, h, radius)
			if alpha == 0.0 {
				continue
			}

			r, g, b, _ := img.At(x, y).RGBA()
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(alpha * 255),
			})
		}
	}
	return dst
}

func roundedRectAlpha(x, y, w, h, r int) float64 {
	samples := 4
	sampleSize := 1.0 / float64(samples)
	hit := 0
	total := samples * samples

	for sy := 0; sy < samples; sy++ {
		for sx := 0; sx < samples; sx++ {

			px := float64(x) + (float64(sx)+0.5)*sampleSize
			py := float64(y) + (float64(sy)+0.5)*sampleSize

			if insideRoundedRect(px, py, w, h, r) {
				hit++
			}
		}
	}
	return float64(hit) / float64(total)
}

func insideRoundedRect(px, py float64, w, h, r int) bool {
	ir := float64(r)
	left := ir
	right := float64(w) - ir
	top := ir
	bottom := float64(h) - ir

	switch {
	case px < left && py < top:
		return dist(px, py, left, top) <= ir
	case px > right && py < top:
		return dist(px, py, right, top) <= ir
	case px < left && py > bottom:
		return dist(px, py, left, bottom) <= ir
	case px > right && py > bottom:
		return dist(px, py, right, bottom) <= ir
	default:

		return px >= 0 && px <= float64(w) && py >= 0 && py <= float64(h)
	}
}

func dist(x1, y1, x2, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return math.Sqrt(dx*dx + dy*dy)
}

func bbfBatch(inputDir string, outputDir string, params Params) int {
	rc := 0
	absInputDir, err := filepath.Abs(inputDir)
	if err != nil {
		fmt.Println("Failed to find absolute input path:", inputDir, err)
		return 1
	}
	if outputDir == "" {
		outputDir = filepath.Join(filepath.Dir(inputDir),
			"bbf_"+filepath.Base(inputDir))
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		fmt.Println("Failed to find absolute output path:", outputDir, err)
		return 1
	}
	err = os.MkdirAll(absOutputDir, 0755)
	if err != nil {
		outputDir, err = filepath.Abs(filepath.Join(inputDir, "../out"))
		fmt.Println("Failed to create output path, relocating to:", outputDir, err)
		if err != nil {
			fmt.Println("Failed to find absolute fallback output path:", outputDir, err)
			return 1
		}
		err = os.MkdirAll(absOutputDir, 0755)
		if err != nil {
			fmt.Println("Failed to create fallback output path:", err)
			return 1
		}
	}
	filepath.WalkDir(absInputDir, func(inputFile string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if err != nil {
			rc = 2
			fmt.Println("Error while walking the directory tree:", err)
			return nil
		}
		outputFile := strings.Replace(inputFile, inputDir, outputDir, 1)
		params.input = inputFile
		params.output = outputFile
		if bbf(params) != 0 {
			rc = 2
		}
		return nil
	})

	return rc
}

func main() {
	input := flag.String("input", "", "Path to input image")
	output := flag.String("output", "", "Path to output image")
	inputDir := flag.String("input-dir", "", "Path to input dir")
	outputDir := flag.String("output-dir", "", "Path to output dir")
	blur := flag.Float64("blur", 20.0, "Blur radius for background")
	width := flag.Int("width", 1920, "Output image width")
	height := flag.Int("height", 1080, "Output image height")
	radius := flag.Int("radius", 20, "Overlay corner radius")
	margin := flag.Int("margin", 20, "Overlay margin")

	flag.Parse()

	params := Params{
		input:  *input,
		output: *output,
		blur:   *blur,
		width:  *width,
		height: *height,
		radius: *radius,
		margin: *margin,
	}

	inputUsed := params.input != ""
	inputDirUsed := *inputDir != ""

	if inputUsed && inputDirUsed {
		fmt.Println("You may only use --input OR --input-dir")
		os.Exit(1)
	} else if !inputUsed && !inputDirUsed {
		fmt.Println("You must use --input OR --input-dir")
		os.Exit(1)
	} else if inputUsed {
		fileInfo, err := os.Stat(params.input)
		if err != nil || fileInfo.IsDir() {
			fmt.Println("Not a file / does not exist:", params.input)
			os.Exit(1)
		}
		os.Exit(bbf(params))
	} else if inputDirUsed {
		fileInfo, err := os.Stat(*inputDir)
		if err != nil || !fileInfo.IsDir() {
			fmt.Println("Not a directory / does not exist:", *inputDir)
			os.Exit(1)
		}
		os.Exit(bbfBatch(*inputDir, *outputDir, params))
	} else {
		fmt.Println("You've somehow broken the fundamental axioms of logic itself, congratulations!")
		os.Exit(1)
	}
}
