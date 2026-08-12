package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

func main() {
	source, err := openPNG(filepath.Join("web", "MeerKit.png"))
	if err != nil {
		log.Fatal(err)
	}
	pluginMark, err := openPNG(filepath.Join("browser-extension", "assets", "plugin-package.png"))
	if err != nil {
		log.Fatal(err)
	}
	pluginMark = extractDarkMark(pluginMark)
	if err := os.MkdirAll(filepath.Join("browser-extension", "icons"), 0o755); err != nil {
		log.Fatal(err)
	}
	for _, size := range []int{16, 32, 48, 128, 512} {
		canvas := resize(source, size)
		drawBadge(canvas, pluginMark)
		path := filepath.Join("browser-extension", "icons", "browser-agent-"+itoa(size)+".png")
		if err := writePNG(path, canvas); err != nil {
			log.Fatal(err)
		}
	}
}

func drawBadge(canvas *image.NRGBA, pluginMark image.Image) {
	size := canvas.Bounds().Dx()
	badgeSize := max(6, int(math.Round(float64(size)*0.31)))
	margin := max(1, int(math.Round(float64(size)*0.045)))
	border := max(1, int(math.Round(float64(size)*0.024)))
	radius := max(1, int(math.Round(float64(badgeSize)*0.20)))
	left := size - margin - badgeSize
	top := size - margin - badgeSize

	fillRoundRect(canvas, left-border, top-border, left+badgeSize+border, top+badgeSize+border, radius+border, color.NRGBA{R: 24, G: 24, B: 27, A: 250})
	fillRoundRect(canvas, left, top, left+badgeSize, top+badgeSize, radius, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
	padding := max(1, int(math.Round(float64(badgeSize)*0.16)))
	mark := resize(pluginMark, badgeSize-padding*2)
	draw.Draw(canvas, image.Rect(left+padding, top+padding, left+padding+mark.Bounds().Dx(), top+padding+mark.Bounds().Dy()), mark, image.Point{}, draw.Over)
}

func fillRoundRect(target draw.Image, left, top, right, bottom, radius int, fill color.Color) {
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			dx := max(max(left+radius-x, 0), x-(right-radius-1))
			dy := max(max(top+radius-y, 0), y-(bottom-radius-1))
			if dx*dx+dy*dy <= radius*radius {
				target.Set(x, y, fill)
			}
		}
	}
}

func resize(source image.Image, size int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	bounds := source.Bounds()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := float64(bounds.Min.X) + (float64(x)+0.5)*float64(bounds.Dx())/float64(size) - 0.5
			sy := float64(bounds.Min.Y) + (float64(y)+0.5)*float64(bounds.Dy())/float64(size) - 0.5
			result.SetNRGBA(x, y, sampleBilinear(source, sx, sy))
		}
	}
	return result
}

func extractDarkMark(source image.Image) *image.NRGBA {
	bounds := source.Bounds()
	result := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA)
			luminance := (uint16(pixel.R)*54 + uint16(pixel.G)*183 + uint16(pixel.B)*19) / 256
			alpha := uint8((uint16(255-luminance) * uint16(pixel.A)) / 255)
			result.SetNRGBA(x-bounds.Min.X, y-bounds.Min.Y, color.NRGBA{R: 39, G: 39, B: 42, A: alpha})
		}
	}
	return result
}

func sampleBilinear(source image.Image, x, y float64) color.NRGBA {
	bounds := source.Bounds()
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := x0+1, y0+1
	x0, x1 = clamp(x0, bounds.Min.X, bounds.Max.X-1), clamp(x1, bounds.Min.X, bounds.Max.X-1)
	y0, y1 = clamp(y0, bounds.Min.Y, bounds.Max.Y-1), clamp(y1, bounds.Min.Y, bounds.Max.Y-1)
	tx, ty := x-math.Floor(x), y-math.Floor(y)
	colors := []color.NRGBA{color.NRGBAModel.Convert(source.At(x0, y0)).(color.NRGBA), color.NRGBAModel.Convert(source.At(x1, y0)).(color.NRGBA), color.NRGBAModel.Convert(source.At(x0, y1)).(color.NRGBA), color.NRGBAModel.Convert(source.At(x1, y1)).(color.NRGBA)}
	weights := []float64{(1 - tx) * (1 - ty), tx * (1 - ty), (1 - tx) * ty, tx * ty}
	var red, green, blue, alpha float64
	for index, value := range colors {
		red += float64(value.R) * weights[index]
		green += float64(value.G) * weights[index]
		blue += float64(value.B) * weights[index]
		alpha += float64(value.A) * weights[index]
	}
	return color.NRGBA{R: uint8(math.Round(red)), G: uint8(math.Round(green)), B: uint8(math.Round(blue)), A: uint8(math.Round(alpha))}
}

func openPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}
func writePNG(path string, value image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, value)
}
func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
