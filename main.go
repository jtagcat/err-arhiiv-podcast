package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/eduncan911/podcast"
	cs "github.com/jtagcat/composedscrape"
	"github.com/jtagcat/util/std"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	dir := os.Args[1]          // /var/www/näide.ee/taskuhäälingud/
	reachableDir := os.Args[2] // https://näide.ee/taskuhäälingud/
	startlink := os.Args[3]    // https://arhiiv.err.ee/seeria/tahelaev/0/0/default/koik
	name := os.Args[4]

	sc := cs.InitScraper(ctx, &cs.Scraper{
		InitGlobalConcurrentLimit: 3,
	})

	doc, _, err := sc.Get(startlink, "#page")
	if err != nil {
		panic(err)
	}

	pod := podcast.New(doc.Find("h1").Text(), startlink, "", nil, nil)
	pod.AddCategory("History", nil)

	for _, listing := range cs.RawEach(doc.Find(".content")) {
		listing := listing
		func() {
			var item podcast.Item

			link, ok := listing.Find("h2 > a").Attr("href")
			if !ok {
				panic(listing)
			}
			link, err = cs.URLJoin(startlink, link)
			if err != nil {
				panic(err)
			}

			date := strings.TrimFunc(listing.Find("#fileDateInList").Text(), func(r rune) bool {
				switch r {
				case '(', ')':
					return true
				default:
					return false
				}
			})
			parsed, err := time.Parse("02.01.2006", date)
			if err != nil {
				panic(err)
			}
			item.PubDate = &parsed

			page, _, err := sc.Get(link, "#page")
			if err != nil {
				panic(err)
			}

			item.Title = page.Find("h1").First().Text()
			item.Description = page.Find(".col2 > p").First().Text()

			script := page.Find(".videoBox").Find("script").Text()
			_, m3u8, ok := strings.Cut(script, "var src = '")
			if !ok {
				panic("no m3u8")
			}
			m3u8, _, ok = strings.Cut(m3u8, "';\n")
			if !ok {
				panic("no termination after m3u8")
			}
			m3u8, err = cs.URLJoin(startlink, m3u8)
			if err != nil {
				panic(err)
			}

			_, filename, ok := std.RevCut(strings.TrimSuffix(m3u8, "/master.m3u8"), "/")
			if !ok {
				panic("no slash in m3u8 url")
			}
			filename, _, ok = std.RevCut(filename, ".") // no file ext
			if !ok {
				panic("no slash in m3u8 url")
			}
			item.GUID = filename
			filename += ".mp3"
			filepath := filepath.Join(dir, filename)

			log.Printf("downloading %s", m3u8)
			if err := exec.CommandContext(ctx, "yt-dlp", "-f", "bestaudio", "-o", filepath, "--", m3u8).Run(); err != nil {
				panic(err)
			}

			stat, err := os.Stat(filepath)
			if err != nil {
				panic(err)
			}

			reachURL, err := cs.URLJoin(reachableDir, name, filename)
			if err != nil {
				panic(err)
			}
			item.AddEnclosure(reachURL,
				podcast.MP3, stat.Size())

			pod.AddItem(item)
		}()
	}

	if err := os.WriteFile(filepath.Join(dir, name+".xml"), pod.Bytes(), 0o600); err != nil {
		panic(err)
	}

	log.Printf(path.Join(reachableDir, name, name+".xml"))
}
