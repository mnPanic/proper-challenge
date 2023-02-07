package imgfinder_test

import (
	"cat-scraper/internal/imgfinder"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// To test colly we need to run a test http server that returns an HTML
// Idea taken from https://github.com/gocolly/colly/blob/master/colly_test.go

func TestCheezburgerScrapperReturnsFullImageLinksWithoutSlugs(t *testing.T) {
	server := NewTestServer([]string{
		// Should be ignored
		`<img class="lazyload" src="data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEAAAAALAAAAAABAAEAAAI=" data-src="https://i.chzbgr.com/s/unversioned/images/logos/IcanHas_logo_ol.png" alt="I Can Has Cheezburger?" title="I Can Has Cheezburger?">`,
		// Normal image to take url from src
		`<img class="resp-media" src="https://i.chzbgr.com/thumb800/19253253/hAA5939B8/gifted-a-baby-voidling-to-my-wife-to-be-right-before-the-ceremony-worked-out-well-ugooosejuice" alt="collection of black cat appreciation posts | thumbnail includes a picture of a bride and groom with the bride holding a tiny black kitten &#39;Gifted a baby voidling to my wife to be right before the ceremony. Worked out well! u/goooseJuice&#39;" title="Black Cat Appreciation Posts: Giving Love To The Underappreciated Voids And Black Holes " width="800" height="420"/>`,
		// Lazy loaded image to take url from data-src
		`<img class='resp-media lazyload' src="data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEAAAAALAAAAAABAAEAAAI=" data-src='https://i.chzbgr.com/full/9732390400/h07F891DD/burn' id='_r_a_9732390400' width="500" height="375" alt="Cheezburger Image 9732390400" title="Burn" /> <noscript> <img class='resp-media' src='https://i.chzbgr.com/full/9732390400/h07F891DD/burn' id='_r_a_9732390400' width="500" height="375" alt="Cheezburger Image 9732390400" title="Burn" />`,
	})

	scrapper := imgfinder.CheezburgerScrapper{}

	urls, err := scrapper.CollectImageURLsFrom(server.URL)
	require.NoError(t, err)

	expectedURLs := []string{
		"https://i.chzbgr.com/full/19253253/hAA5939B8",   // changed from thumb800 to full and removed slug
		"https://i.chzbgr.com/full/9732390400/h07F891DD", // removed slug
	}

	assert.Equal(t, expectedURLs, urls)
}

func TestCheezburgerScrapperInvalidURLs(t *testing.T) {
	server := NewTestServer([]string{
		// URL has no slug
		`<img class="resp-media" src="https://i.chzbgr.com/full/9732390400/h07F891DD" alt="collection of black cat appreciation posts | thumbnail includes a picture of a bride and groom with the bride holding a tiny black kitten &#39;Gifted a baby voidling to my wife to be right before the ceremony. Worked out well! u/goooseJuice&#39;" title="Black Cat Appreciation Posts: Giving Love To The Underappreciated Voids And Black Holes " width="800" height="420"/>`,
	})

	scrapper := imgfinder.CheezburgerScrapper{}

	_, err := scrapper.CollectImageURLsFrom(server.URL)
	require.EqualError(t, err, "can't get full size version of 'https://i.chzbgr.com/full/9732390400/h07F891DD': unexpected path format, expected {size}/{id1}/{id2}/{slug}")
}

func NewTestServer(images []string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<title>Test Page</title>
</head>
<body>
%s
</body>
</html>`, strings.Join(images, " "))))
	}))

	return server
}
