package main

import (
	"cat-scraper/cmd/cli"
	"cat-scraper/internal/imgfinder"
	"log"
	"net/http"
)

func main() {
	finder := imgfinder.New(
		imgfinder.CheezburgerScrapper{},
		imgfinder.RealFileSystem{},
		http.DefaultClient,
	)

	err := cli.Run(finder)
	if err != nil {
		log.Fatal(err)
	}
}
