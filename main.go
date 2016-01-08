package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/odeke-em/extrict/src"
	"github.com/odeke-em/vidler/downloader"

	"github.com/odeke-em/extractor"
)

var envKeyAlias = &extractor.EnvKey{
	PubKeyAlias:  "VIDLER_PUB_KEY",
	PrivKeyAlias: "VIDLER_PRIV_KEY",
}

var envKeySet = extractor.KeySetFromEnv(envKeyAlias)

func actionableLinkForm(uri string) string {
	return fmt.Sprintf(
		`
        <html>
            <title>Extract videos from a link</title>
            <body>
                <form action=%q method="POST">
                    <label name="uri_label">URI with media of extensions mp4, gif, webm to crawl </label>
                    <input name="uri" value="https://vine.co/channels/comedy"></input>
                    <br />
                    <button type="submit">Submit</button>
                </form>
            </body>
        </html>
    `, uri)
}

func uriInsertions(w io.Writer, ut downloader.UriInsert) {
	t := template.New("newiters")
	t = t.Funcs(template.FuncMap{
		"basename": filepath.Base,
		"sign": func(uri string) string {
			return fmt.Sprintf("%s", envKeySet.Sign([]byte(uri)))
		},
		"pubkey": func() string {
			return envKeySet.PublicKey
		},
		"contentType": func() string {
			return ut.ContentType
		},
	})

	t, _ = t.Parse(
		`
    {{ range .UriList }}
        <video width="70%" controls>
            <source src="{{ . }}" type="{{ contentType }}">{{ basename . }}</source>
        </video>
        <br />
        <a href="/download?uri={{ . }}&signature={{ sign . }}&pubkey={{ pubkey }}">Download</a>
        <br />
        <br />
    {{ end }}
    `)
	t.Execute(w, ut)
}

func requestDownloadForm(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, actionableLinkForm("/extrict/mp4"))
}

func requestDownloadFormGif(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, actionableLinkForm("/extrict/gif"))
}

func requestDownloadFormWebm(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, actionableLinkForm("/extrict/webm"))
}

func requestDownloadFormPng(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, actionableLinkForm("/extrict/png"))
}

func download(di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	if di.PublicKey != envKeySet.PublicKey {
		http.Error(res, "invalid publickey", 400)
		return
	}

	if !envKeySet.Match([]byte(di.URI), []byte(di.Signature)) {
		http.Error(res, "invalid signature", 400)
		return
	}

	downloader.Download(di.URI, res, req)
}

var extensionToMimeType = map[string]string{
	"mp4":  "video/mp4",
	"gif":  "image/gif",
	"webm": "video/webm",
	"mp3":  "audio/mp3",
}

func extrictMp4(di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	extrictMedia("mp4", di, res, req)
}

func extrictGif(di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	extrictMedia("gif", di, res, req)
}

func extrictWebm(di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	extrictMedia("webm", di, res, req)
}

func extrictPng(di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	extrictMedia("png", di, res, req)
}

func extrictMedia(extensionKey string, di downloader.DownloadItem, res http.ResponseWriter, req *http.Request) {
	hits := extrict.CrawlAndMatchByExtension(di.URI, extensionKey, 1)
	// fmt.Println("di.URI", di.URI)
	cache := map[string]bool{}

	hitList := []string{}

	for hit := range hits {
		if _, ok := cache[hit]; ok {
			continue
		}

		hitList = append(hitList, hit)
		cache[hit] = true
	}

	contentType, ok := extensionToMimeType[extensionKey]
	if !ok {
		contentType = "application/octet-stream"
	}

	// fmt.Println(hitList)
	fmt.Fprintf(res, `
    <html>
        <body>
    `)

	hitCount := len(hitList)

	if hitCount < 1 {
		fmt.Fprintf(res, `<p> No hits found for %q`, di.URI)
	} else {
		plurality := "hit"
		if hitCount != 1 {
			plurality = "hits"
		}

		fmt.Fprintf(res, `
            <h4>%v %v for </h4> <a href=%q>%v</a>
            <br />
        `, hitCount, plurality, di.URI, di.URI)

		uriInsertions(res, downloader.UriInsert{UriList: hitList, Source: di.URI, ContentType: contentType})
	}

	fmt.Fprintf(res,
		`
            <a href="/">Go back</a>
            </body>
        </html>
    `)
}

func errorPrint(fmt_ string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\033[31m")
	fmt.Fprintf(os.Stderr, fmt_, args...)
	fmt.Fprintf(os.Stderr, "\033[00m")
}

func main() {
	if envKeySet.PublicKey == "" {
		errorPrint("publicKey not set. Please set %s in your env.\n", envKeyAlias.PubKeyAlias)
		return
	}

	if envKeySet.PrivateKey == "" {
		errorPrint("privateKey not set. Please set %s in your env.\n", envKeyAlias.PrivKeyAlias)
		return
	}

	m := martini.Classic()

	m.Get("/", requestDownloadForm)
	m.Get("/gif", requestDownloadFormGif)
	m.Get("/webm", requestDownloadFormWebm)
	m.Get("/png", requestDownloadFormPng)
	m.Get("/head", binding.Bind(downloader.DownloadItem{}), downloader.HeadGet)
	m.Get("/download", binding.Bind(downloader.DownloadItem{}), download)

	m.Post("/extrict/gif", binding.Bind(downloader.DownloadItem{}), extrictGif)
	m.Post("/extrict/mp4", binding.Bind(downloader.DownloadItem{}), extrictMp4)
	m.Post("/extrict/webm", binding.Bind(downloader.DownloadItem{}), extrictWebm)
	m.Post("/extrict/png", binding.Bind(downloader.DownloadItem{}), extrictPng)
	m.Post("/download", binding.Bind(downloader.DownloadItem{}), download)

	m.Run()
}
