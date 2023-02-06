# Proper Case Challenge

Case Challenge for Proper, part of the interview process

Participant information

- Name: Manuel Panichelli
- Email: panicmanu@gmail.com
- LinkedIn: [/in/manuel-panichelli/](https://www.linkedin.com/in/manuel-panichelli/)

## Case 1

> **Assignment**: Write a program that downloads the images from
http://icanhas.cheezburger.com/ and stores them locally. Only download graphics
that are memes on the page and not sponsored content. Download the first ten
memes on the homepage and store them locally. Name them `1.jpg`, ..., `10.jpg`.

I started out by inspecting the page, and noticed that memes are all `img`
elements with the image urls on the `src` tags.

When I tried scraping the images, I'd get weird URLs in the `src` tag like
`data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEAAAAALAAAAAABAAEAAAI=`. I realized
that's because it lazy loads images, and the `src` tag starts out with a dummy
value which is replaced with `data-src` later. So I considered both.

I found the scraping framework [`colly`](http://go-colly.org/) which seems easy
enough to use, so I'll use that from now on.

## References

- Web Scraping
  - https://www.scrapingbee.com/blog/web-scraping-go/
  - http://go-colly.org/
- DOM: https://developer.mozilla.org/en-US/docs/Web/API/Document_Object_Model/Introduction
- CSS selectors: https://www.w3schools.com/cssref/css_selectors.php