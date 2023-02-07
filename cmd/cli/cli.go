package cli

import (
	"cat-scraper/internal/imgfinder"
	"flag"
	"fmt"
)

// Command line flags
var (
	amount  = flag.Int("amount", 10, "how many memes to download")
	threads = flag.Int("threads", 1, "number of threads that will download images concurrently (max: 5)")
)

func Run(finder imgfinder.Finder) error {
	flag.Parse()
	fmt.Printf("Downloading %d memes with %d threads\n", *amount, *threads)

	const imagesDirectory = "images/"
	err := finder.CollectAndDownloadImages(*amount, *threads, imagesDirectory)
	if err != nil {
		return err
	}

	fmt.Println("Images saved successfully")
	return nil
}
