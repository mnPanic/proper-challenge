package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocolly/colly"
)

func main() {
	err := collectAndDownloadImages()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Images saved successfully")
}

func collectAndDownloadImages() error {
	urls := collectImageURLsFrom("https://icanhas.cheezburger.com/")
	fmt.Printf("Found %d images\n", len(urls))

	// Assume that there will always be at least 10 memes on the home page
	err := downloadImages(urls[0:10], "images/")
	if err != nil {
		return fmt.Errorf("downloading images: %s", err)
	}

	return nil
}

func downloadImages(urls []string, basePath string) error {
	for i, url := range urls {
		name := fmt.Sprintf("%d.jpg", i+1) // i+1 to number from 1 and not 0
		path := filepath.Join(basePath, name)

		err := downloadImage(url, path)
		if err != nil {
			return fmt.Errorf("downloading image #%d: %s", i, err)
		}
	}

	return nil
}

func downloadImage(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get: %s", err)
	}

	// TODO: Consider using io.Copy and opening the file manually to avoid
	// having the body twice in memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %s", err)
	}

	// Permissions don't matter much here
	err = os.WriteFile(filename, body, 0777)
	if err != nil {
		return fmt.Errorf("saving: %s", err)
	}

	return nil
}

func collectImageURLsFrom(pageURL string) []string {
	var imgURLs []string

	c := colly.NewCollector()

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	// Select all img elements and save their source
	c.OnHTML(`img`, func(e *colly.HTMLElement) {
		// Memes have classes resp-media (for content that loads with the page)
		// and resp-media lazyload (for lazy loaded content). The rest are not
		// memes.
		imgClass := e.Attr("class")
		if !(imgClass == "resp-media" || imgClass == "resp-media lazyload") {
			return
		}

		// Because the page lazy loads images and only loads them when they
		// appear on the viewport, they start out with the src in `data-src`
		// while `src` has a placeholder value (that's not an url starting
		// with https)
		imgURL := e.Attr("src")
		if !strings.HasPrefix(imgURL, "https") {
			imgURL = e.Attr("data-src")
		}

		//fmt.Printf("class: %s\n", e.Attr("class"))
		//fmt.Println("Image link found", imgURL)
		imgURLs = append(imgURLs, imgURL)
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	c.Visit(pageURL)

	return imgURLs
}
