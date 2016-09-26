package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const proxyType = "http"

var (
	port        = flag.Int("p", 3000, "proxy port")
	host        = flag.String("h", "http://www.baikal.travel/", "target host")
	lookFor     = flag.String("s", "Байкал", "substring for replacement")
	replaceWith = flag.String("r", "Baikal", "substring to replace with")
	u           *url.URL
)

func main() {
	flag.Parse()
	u = new(url.URL)
	u.Scheme = proxyType
	u.Host = *host
	log.Printf("Starting...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.Host = u.Host
			r.URL.Scheme = u.Scheme
			r.URL.Host = u.Host
		},
		Transport: &transport{http.DefaultTransport},
	}))
}

type transport struct {
	rt http.RoundTripper
}

func (t *transport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.rt.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if strings.Contains(response.Header.Get("Content-Type"), "text") {
		var reader io.ReadCloser
		switch response.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(response.Body)
			if err != nil {
				return nil, err
			}
		default:
			reader = response.Body
		}
		utf8, err := charset.NewReader(reader, response.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}
		reader = ioutil.NopCloser(utf8)
		nbody := &bytes.Buffer{}
		io.Copy(nbody, reader)
		reader.Close()

		str := string(nbody.Bytes())
		str = strings.Replace(str, *lookFor, *replaceWith, -1)
		nbody.Reset()
		//if original content was not utf8-encoded, in this example only windows-1251 and koi8-r supported
		switch {
		case strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "windows-1251"):
			var encBuf bytes.Buffer
			utf8InWin := transform.NewWriter(&encBuf, charmap.Windows1251.NewEncoder())
			utf8InWin.Write([]byte(str))
			utf8InWin.Close()
			str = encBuf.String()
		case strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "koi8-r"):
			var encBuf bytes.Buffer
			utf8InKoi8r := transform.NewWriter(&encBuf, charmap.KOI8R.NewEncoder())
			utf8InKoi8r.Write([]byte(str))
			utf8InKoi8r.Close()
			str = encBuf.String()
		}
		buf := new(bytes.Buffer)
		switch response.Header.Get("Content-Encoding") {
		case "gzip":
			w := gzip.NewWriter(buf)
			_, err := w.Write([]byte(str))
			if err != nil {
				return nil, err
			}
			w.Close()
		default:
			_, err := buf.Write([]byte(str))
			if err != nil {
				return nil, err
			}
		}
		length := len(buf.Bytes())
		b := ioutil.NopCloser(buf)
		response.Body = b
		response.Body.Close()
		response.ContentLength = int64(length)
		response.Header.Set("Content-Length", strconv.Itoa(length))
	}
	return response, err
}
