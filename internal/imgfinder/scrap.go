package imgfinder

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gocolly/colly"
)

// CheezburgerScrapper scraps images from https://icanhas.cheezburger.com/
// It may return the same images twice for different pages.
type CheezburgerScrapper struct{}

func (s CheezburgerScrapper) CollectImageURLsFrom(pageURL string) ([]string, error) {
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

		imgURLs = append(imgURLs, imgURL)
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	err := c.Visit(pageURL)
	if err != nil {
		return nil, fmt.Errorf("visiting: %s", err)
	}

	return convertToFullSizeURLs(imgURLs)
}

func convertToFullSizeURLs(urls []string) ([]string, error) {
	var fullSizeURLs []string
	for _, url := range urls {
		fullSizeURL, err := getFullSizeVersion(url)
		if err != nil {
			return nil, fmt.Errorf("can't get full size version of '%s': %s", url, err)
		}

		fullSizeURLs = append(fullSizeURLs, fullSizeURL)
	}

	return fullSizeURLs, nil
}

func getFullSizeVersion(imageURL string) (string, error) {
	// Cheezburger image urls have the following format
	//
	//	https://i.chzbrg.com/{size}/{id1}/{id2}/{slug}
	//
	// We want the full size version and not a downscaled thumbnail. This will
	// also help prevent downloading the same images more than once.
	//
	// Some examples:
	// - https://i.chzbgr.com/full/9730332160/h6860EF7A/just-no
	// - https://i.chzbgr.com/thumb1200/19206661/h5E69E7B5/feral-trapped-last-week-he-has-strong-feelings-about-domestication-me-and-the-horse-i-rode-in-on
	//
	// The slug doesn't matter, and sometimes slugs are different (so we
	// download them differently). To avoid that, also remove the slug.

	url, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("parse: %s", err)
	}

	// Trim the leading / and split by / to separate the size from the rest of
	// the path so we can replace it.
	parts := strings.Split(strings.TrimPrefix(url.Path, "/"), "/")
	if len(parts) != 4 {
		return "", errors.New("unexpected path format, expected {size}/{id1}/{id2}/{slug}")
	}

	// Replace size for full and remove the slug
	url.Path = fmt.Sprintf("full/%s/%s", parts[1], parts[2])

	return url.String(), nil
}
