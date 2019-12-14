package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"time"
)

func main() {
	image1, err := os.Open("./images/2.png")
	if err != nil {
		log.Fatalf("failed to open: %s", err)
	}

	image2, err := os.Open("./images/1.png")
	if err != nil {
		log.Fatalf("failed to open: %s", err)
	}
	defer image1.Close()
	defer image2.Close()

	now := time.Now()

	first, err := png.Decode(image1)
	if err != nil {
		log.Fatalf("failed to decode: %s", err)
	}

	second, err := png.Decode(image2)
	if err != nil {
		log.Fatalf("failed to decode: %s", err)
	}

	offset := image.Pt(0, 0)
	b := first.Bounds()
	image3 := image.NewRGBA(b)
	draw.Draw(image3, b, first, image.ZP, draw.Src)
	draw.Draw(image3, second.Bounds().Add(offset), second, image.ZP, draw.Over)

	fmt.Println(time.Since(now))
	third, err := os.Create("result.png")
	if err != nil {
		log.Fatalf("failed to create: %s", err)
	}
	png.Encode(third, image3)
	defer third.Close()
}
