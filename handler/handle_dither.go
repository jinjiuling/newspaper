package handler

import (
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"strconv"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	dither "github.com/esimov/dithergo"
	"github.com/nfnt/resize"
)

func handleDitherImage(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "missing url param", 400)
		return
	}

	res, err := c.Get(url)
	if err != nil {
		log.Printf("Error fetching image: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer res.Body.Close()

	img, _, err := image.Decode(res.Body)
	if err != nil {
		log.Printf("Error decoding image: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if width == 0 {
		width = 600
	}

	img = resize.Resize(uint(width), 0, img, resize.Lanczos3)

	d := dither.Dither{
		Type: "FloydSteinberg",
		Settings: dither.Settings{
			Filter: [][]float32{
				{0.0, 0.0, 0.0, 7.0 / 48.0, 5.0 / 48.0},
				{3.0 / 48.0, 5.0 / 48.0, 7.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0},
				{1.0 / 48.0, 3.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0, 1.0 / 48.0},
			},
		},
	}

	result := d.Monochrome(img, 1)

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	jpeg.Encode(w, result, &jpeg.Options{Quality: 80})
}

func handleQRCode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id param", 400)
		return
	}

	b, err := qr.Encode(fmt.Sprintf("https://news.russellsaw.io/?id=%s", id), qr.M, qr.Auto)
	if err != nil {
		log.Printf("Error encoding QR: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	b, _ = barcode.Scale(b, 100, 100)

	w.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(w, b, &jpeg.Options{Quality: 90})
}