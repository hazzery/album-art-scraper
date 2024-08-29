package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

func downloadAlbumArt(albumArtLink string, albumTitle string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	response, err := http.Get(albumArtLink)
	if err != nil {
		fmt.Printf("Error making http request: %s\n", err)
	}
	defer response.Body.Close()

	albumArt, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading request body %s\n", err)
	}
	err = os.WriteFile("album_arts"+albumTitle+".jpg", albumArt, 0666)
	if err != nil {
		fmt.Printf("Error writing file: %s\n", err)
	}
	// fmt.Printf("Downloaded %s\n", albumTitle)
}

func getNodeAttr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func parseAlbumPage(node *html.Node, albumData map[string]string) {
	if node.Type == html.ElementNode && node.Data == "meta" {
		propertyValue := getNodeAttr(node, "property")

		if propertyValue == "og:image" || propertyValue == "og:title" {
			albumData[propertyValue] = getNodeAttr(node, "content")

			if len(albumData) == 2 {
				return
			}
		}
	}

	// traverse the child nodes
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parseAlbumPage(child, albumData)
	}
}

func fetchAlbumPage(albumPageLink string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	response, err := http.Get(albumPageLink)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	document, err := html.Parse(response.Body)
	if err != nil {
		fmt.Printf("error parsing html: %s\n", err)
		os.Exit(1)
	}
	albumData := map[string]string{}
	parseAlbumPage(document, albumData)

	if len(albumData) != 2 {
		fmt.Printf("Album page didn't contain necessary data! %s\n", albumPageLink)
		return
	}
	waitGroup.Add(1)
	go downloadAlbumArt(albumData["og:image"], albumData["og:title"], waitGroup)
}

func main() {
	linksFile, err := os.ReadFile("links.txt")
	if err != nil {
		log.Fatal(err)
	}
	allLinks := strings.TrimSuffix(string(linksFile), "\n")
	links := strings.Split(allLinks, ", ")

	var waitGroup sync.WaitGroup

	// waitGroup.Add(1)
	// go fetchAlbumPage(links[0], &waitGroup)

	waitGroup.Add(len(links))
	for _, link := range links {
		go fetchAlbumPage(link, &waitGroup)
	}

	waitGroup.Wait()
}
