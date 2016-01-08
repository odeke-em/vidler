package downloader

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

type DownloadItem struct {
	URI       string `form:"uri" binding:"required"`
	PublicKey string `form:"pubkey" binding:"-"`
	Signature string `form:"signature" binding:"-"`
}

type UriInsert struct {
	UriList []string
	Source  string
	ContentType string
}

func headerShallowCopy(from, to http.Header) {
	for k, v := range from {
		to.Set(k, strings.Join(v, ","))
	}
}

func HeadGet(uri string, res http.ResponseWriter, req *http.Request) error {
	headResponse, err := http.Head(uri)

	if err != nil {
		return err
	}

	dlHeader := headResponse.Header
	headerShallowCopy(dlHeader, res.Header())

	return nil
}

func Download(uri string, res http.ResponseWriter, req *http.Request) {
	downloadResult, err := http.Get(uri)

	if err != nil {
		fmt.Fprintf(res, "%v", err)
		return
	}

	if downloadResult == nil || downloadResult.Body == nil {
		fmt.Fprintf(res, "could not get %q", uri)
		return
	}

	body := downloadResult.Body
	dlHeader := downloadResult.Header

	if downloadResult.Close {
		defer body.Close()
	}

	headerShallowCopy(dlHeader, res.Header())

	basename := filepath.Base(uri)
	extraHeaders := map[string][]string{
		"Content-Disposition": []string{
			fmt.Sprintf("attachment;filename=%q", basename),
		},
	}

	headerShallowCopy(extraHeaders, res.Header())

	res.WriteHeader(200)
	io.Copy(res, body)
}
