package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocolly/colly"
)

var amount = flag.Int("amount", 10, "how many memes to download")

func main() {
	flag.Parse()
	fmt.Printf("Downloading %d memes\n", *amount)
	err := collectAndDownloadImages(*amount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Images saved successfully")
}

func collectAndDownloadImages(amount int) error {
	var imageURLs []string
	currentPage := 1
	for len(imageURLs) < amount {
		urls := collectImageURLsFrom(cheezburgerURLForPage(currentPage))
		fmt.Printf("Found %d images\n", len(urls))

		imageURLs = append(imageURLs, urls...)
		currentPage++
	}

	err := downloadImages(imageURLs[0:amount], "images/")
	if err != nil {
		return fmt.Errorf("downloading images: %s", err)
	}

	return nil
}

func cheezburgerURLForPage(pageNumber int) string {
	if pageNumber == 1 {
		return "https://icanhas.cheezburger.com/"
	}

	return fmt.Sprintf("https://icanhas.cheezburger.com/page/%d", pageNumber)
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
